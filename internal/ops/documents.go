/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 *
 * uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * MIT License for more details.
 *
 * You should have received a copy of the MIT License
 * along with uruflow. If not, see the LICENSE file in the project root.
 */

package ops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/observe"
	"github.com/mustafanass/uruflow/internal/projects"
	"github.com/mustafanass/uruflow/internal/storage"
)

var ErrConflict = errors.New("this document changed since you opened it; reload before saving")

type Document struct {
	Content  string `json:"content"`
	Revision string `json:"revision"`
}

func document(content string) Document {
	hash := sha256.Sum256([]byte(content))
	return Document{content, hex.EncodeToString(hash[:])}
}
func (e *Engine) ProjectDocument(name string, variables bool) (Document, error) {
	project, err := e.server.Store().GetProject(name)
	if err != nil {
		return Document{}, err
	}
	if !project.Managed() || project.Env == "" {
		return Document{}, fmt.Errorf("project %s has no authoritative environment file", name)
	}
	if variables {
		_, path := e.server.Loader().Paths(project.Base(), project.Env)
		raw, _, err := readOptionalFile(path)
		if err != nil {
			return Document{}, err
		}
		content, err := projects.FormatVariableEditor(raw)
		return document(content), err
	}
	raw, err := os.ReadFile(project.Source)
	return document(string(raw)), err
}
func (e *Engine) SaveProjectDocument(name string, variables bool, content, revision string, policy func(models.Project) error) (Document, error) {
	e.configurationMu.Lock()
	defer e.configurationMu.Unlock()
	finish, maintenanceErr := e.server.Pipeline().BeginRegistryUpdate()
	if maintenanceErr != nil {
		return Document{}, maintenanceErr
	}
	defer finish()
	current, err := e.ProjectDocument(name, variables)
	if err != nil {
		return Document{}, err
	}
	if revision == "" || current.Revision != revision {
		return Document{}, ErrConflict
	}

	project, err := e.server.Store().GetProject(name)
	if err != nil {
		return Document{}, err
	}
	configuration := content
	var variableContent *string
	if variables {
		raw, readErr := os.ReadFile(project.Source)
		if readErr != nil {
			return Document{}, readErr
		}
		configuration = string(raw)
		edit, parseErr := projects.ParseVariableEditor(content, name)
		if parseErr != nil {
			return Document{}, parseErr
		}
		variableContent = &edit.DotEnv
	}
	preview, err := e.server.Loader().Preview(project.Source, configuration, variableContent)
	if err != nil {
		return Document{}, err
	}
	if policy != nil {
		if err = policy(*preview); err != nil {
			return Document{}, err
		}
	}
	if variables {
		err = e.projectVariables(name, content)
	} else {
		if strings.TrimSpace(content) == "" {
			return Document{}, errors.New("project configuration cannot be empty")
		}
		err = e.editProject(name, content)
	}
	if err != nil {
		return Document{}, err
	}
	return e.ProjectDocument(name, variables)
}
func (e *Engine) CreateProject(name, environment, content string) (*models.Project, error) {
	e.configurationMu.Lock()
	defer e.configurationMu.Unlock()
	finish, maintenanceErr := e.server.Pipeline().BeginRegistryUpdate()
	if maintenanceErr != nil {
		return nil, maintenanceErr
	}
	defer finish()
	if err := e.createProject(name, environment, content); err != nil {
		return nil, err
	}
	p, err := e.server.Store().GetProject(name + "-" + environment)
	if err != nil {
		return nil, err
	}
	safe := observe.RedactProject(*p)
	return &safe, nil
}
func (e *Engine) ReloadProjects() []string {
	e.configurationMu.Lock()
	defer e.configurationMu.Unlock()
	finish, maintenanceErr := e.server.Pipeline().BeginRegistryUpdate()
	if maintenanceErr != nil {
		return []string{maintenanceErr.Error()}
	}
	defer finish()
	e.server.ReloadProjects()
	var problems []string
	for _, p := range e.server.ProjectProblems() {
		problems = append(problems, p.Path+": "+p.Reason.Error())
	}
	return problems
}
func (e *Engine) Deploy(name, sourceRelease string, rollback, rebuild bool) (*models.Release, error) {
	var release *models.Release
	var err error
	if rollback {
		if sourceRelease != "" {
			release, err = e.server.Pipeline().RollbackRelease(name, sourceRelease)
		} else {
			release, err = e.server.Pipeline().Rollback(name, "")
		}
	} else {
		release, err = e.server.Pipeline().Trigger(name, nil, models.TriggerManual, rebuild)
	}
	if err != nil {
		return nil, err
	}
	return e.Release(release.ID)
}
func (e *Engine) StopProject(name string) error { return e.server.Pipeline().Stop(name) }
func (e *Engine) DeleteProject(name string, volumes, images bool) error {
	e.configurationMu.Lock()
	defer e.configurationMu.Unlock()
	return e.server.Pipeline().Delete(name, volumes, images, e.deleteProject)
}
func (e *Engine) Images(ctx context.Context) (models.ImageCatalog, error) {
	return e.server.Registry().Images(ctx)
}
func (e *Engine) RemoveImage(ctx context.Context, repository, tag string) error {
	finish, err := e.server.Pipeline().BeginRegistryUpdate()
	if err != nil {
		return err
	}
	defer finish()
	return e.server.Registry().DeleteTag(ctx, repository, tag)
}
func (e *Engine) ResolveAlert(id string) error { return e.server.Store().ResolveAlert(id) }

func (e *Engine) RecentAlerts() ([]models.Alert, error) {
	return e.server.Store().ListRecentAlerts(200)
}

func (e *Engine) Alert(id string) (*models.Alert, error) {
	rows, err := e.server.Store().ListActiveAlerts()
	if err != nil {
		return nil, err
	}
	for _, a := range rows {
		if a.ID == id {
			return &a, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (e *Engine) Image(ctx context.Context, repository, tag string) (models.Image, error) {
	return e.server.Registry().Image(ctx, repository, tag)
}
func (e *Engine) RefreshImages() { e.server.Registry().RefreshCatalog() }
