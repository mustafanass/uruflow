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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/projects"
)

func (e *Engine) createProject(project, environment, input string) error {
	if !models.ValidResourceName(project) || !models.ValidResourceName(environment) {
		return errors.New("project and environment must be lowercase resource names")
	}
	environmentDefinition, err := projects.ParseCreationYAML(input)
	if err != nil {
		return fmt.Errorf("validate environment YAML: %w", err)
	}
	draft := projects.Draft{
		Project: project, Env: environment, Environment: environmentDefinition,
	}
	environmentPath, _ := e.server.Loader().Paths(project, environment)
	_, projectErr := os.Stat(filepath.Dir(environmentPath))
	newProject := errors.Is(projectErr, os.ErrNotExist)
	if projectErr != nil && !newProject {
		return projectErr
	}
	if err := e.server.Loader().Create(draft); err != nil {
		return err
	}
	e.server.ReloadProjects()
	projectDir := filepath.Dir(environmentPath)
	for _, problem := range e.server.ProjectProblems() {
		if problem.Path == environmentPath || newProject && pathWithin(projectDir, problem.Path) {
			_ = e.server.Loader().Remove(project, environment)
			e.server.ReloadProjects()
			return fmt.Errorf("project was not created: %w", problem.Reason)
		}
	}
	return nil
}

func (e *Engine) deleteProject(project *models.Project) error {
	if project.Managed() && project.Env != "" {
		if err := e.server.Loader().Remove(project.Base(), project.Env); err != nil {
			return fmt.Errorf("remove %s files: %w", project.Name, err)
		}
	}
	if err := e.server.Store().DeleteProject(project.Name); err != nil {
		return err
	}
	e.server.ReloadProjects()
	return nil
}

func pathWithin(directory, path string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func (e *Engine) editProject(name, input string) error {
	project, err := e.server.Store().GetProject(name)
	if err != nil {
		return fmt.Errorf("project %s: %w", name, err)
	}
	if !project.Managed() || project.Env == "" {
		return fmt.Errorf("project %s has no authoritative environment file", project.Name)
	}
	if err := e.server.Loader().Validate(project.Source, input); err != nil {
		return fmt.Errorf("validate YAML: %w", err)
	}
	previous, err := os.ReadFile(project.Source)
	if err != nil {
		return err
	}
	if err := e.server.Loader().WriteEnvironment(project.Base(), project.Env, []byte(input)); err != nil {
		return err
	}
	e.server.ReloadProjects()
	for _, problem := range e.server.ProjectProblems() {
		if problem.Path == project.Source {
			restoreErr := e.server.Loader().WriteEnvironment(project.Base(), project.Env, previous)
			e.server.ReloadProjects()
			if restoreErr != nil {
				return fmt.Errorf("YAML was not saved: %v; restore previous YAML: %w", problem.Reason, restoreErr)
			}
			return fmt.Errorf("YAML was not saved: %w", problem.Reason)
		}
	}
	return nil
}
