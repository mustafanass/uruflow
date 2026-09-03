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
	"sort"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/projects"
	"github.com/mustafanass/uruflow/internal/secrets"
)

func (e *Engine) projectVariables(projectName, input string, source bool, emit Emit) error {
	project, err := e.server.Store().GetProject(projectName)
	if err != nil {
		return fmt.Errorf("project %s: %w", projectName, err)
	}
	if !project.Managed() || project.Env == "" {
		return fmt.Errorf("project %s has no authoritative environment file", project.Name)
	}
	environmentPath, variablesPath := e.server.Loader().Paths(project.Base(), project.Env)
	previous, hadPrevious, err := readOptionalFile(variablesPath)
	if err != nil {
		return err
	}
	if source {
		content, err := projects.FormatVariableEditor(previous)
		if err != nil {
			return fmt.Errorf("open variable editor: %w", err)
		}
		return emit(Event{Type: EventResult, Time: time.Now(), Title: "project variables editor", Message: content})
	}

	edit, err := projects.ParseVariableEditor(input, project.Name)
	if err != nil {
		return fmt.Errorf("validate environment: %w", err)
	}
	previousSecrets, err := projects.VariableSecretNames(previous)
	if err != nil {
		return err
	}
	sealed := make(map[string][]byte, len(edit.Secrets))
	for name, value := range edit.Secrets {
		material, err := e.server.Vault().Seal(value)
		if err != nil {
			return fmt.Errorf("seal %s: %w", name, err)
		}
		sealed[name] = material
	}
	if err := writePrivateFile(variablesPath, edit.DotEnv); err != nil {
		return err
	}
	rollback := func(cause error) error {
		var restoreErr error
		if hadPrevious {
			restoreErr = writePrivateFile(variablesPath, previous)
		} else {
			restoreErr = os.Remove(variablesPath)
			if errors.Is(restoreErr, os.ErrNotExist) {
				restoreErr = nil
			}
		}
		e.server.ReloadProjects()
		if restoreErr != nil {
			return fmt.Errorf("%v; restore environment: %w", cause, restoreErr)
		}
		return cause
	}

	loaded := e.server.ReloadProjects()
	for _, problem := range e.server.ProjectProblems() {
		if problem.Path == environmentPath {
			return rollback(fmt.Errorf("environment was not saved: %w", problem.Reason))
		}
	}
	referenced, err := e.referencedSecrets()
	if err != nil {
		return rollback(err)
	}
	remove := unreferenced(previousSecrets, referenced)
	if err := e.server.Store().UpdateSecrets(sealed, remove); err != nil {
		return rollback(fmt.Errorf("store encrypted variables: %w", err))
	}
	rows, err := variableRows(edit.DotEnv)
	if err != nil {
		return err
	}
	if err := emit(Message("success", fmt.Sprintf("saved %s and loaded %d project environments", variablesPath, loaded))); err != nil {
		return err
	}
	return emit(Table(project.Name+" environment", []string{"NAME", "TYPE", "VALUE OR REFERENCE"}, rows))
}

func (e *Engine) referencedSecrets() (map[string]bool, error) {
	values, err := e.server.Store().ListProjects()
	if err != nil {
		return nil, err
	}
	referenced := make(map[string]bool)
	for _, project := range values {
		for _, name := range secrets.Names(project.Runtime.Env) {
			referenced[name] = true
		}
		for _, service := range project.ServiceList() {
			for _, name := range secrets.Names(service.Env) {
				referenced[name] = true
			}
		}
	}
	return referenced, nil
}

func variableRows(raw string) ([][]string, error) {
	values, err := projects.ParseDotEnv(raw)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := make([][]string, 0, len(names))
	for _, name := range names {
		value := values[name]
		kind := "plain"
		if len(secrets.References(value)) > 0 {
			kind = "secret"
		}
		rows = append(rows, []string{name, kind, value})
	}
	return rows, nil
}

func unreferenced(candidates []string, referenced map[string]bool) []string {
	result := make([]string, 0, len(candidates))
	for _, name := range candidates {
		if !referenced[name] {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func readOptionalFile(path string) (string, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return string(content), err == nil, err
}

func writePrivateFile(path, content string) error {
	if strings.TrimSpace(content) == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".uruflow-env-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.WriteString(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
