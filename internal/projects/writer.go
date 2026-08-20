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

package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	definitionMode = 0o644
	secretMode     = 0o600
	directoryMode  = 0o755
)

type Draft struct {
	Project     string
	Env         string
	Definition  Definition
	Environment Environment
	RawYAML     string
	RawEnv      string
}

func (l *Loader) Paths(project, env string) (definition, environment, variables string) {
	dir := filepath.Join(l.dir, project)
	return filepath.Join(dir, ProjectFile),
		filepath.Join(dir, env+YamlSuffix),
		filepath.Join(dir, env+EnvSuffix)
}

func (l *Loader) Write(draft Draft) error {
	if draft.Project == "" {
		return fmt.Errorf("project name is required")
	}
	if draft.Env == "" {
		return fmt.Errorf("environment name is required")
	}
	if err := validName(draft.Project); err != nil {
		return err
	}
	if err := validName(draft.Env); err != nil {
		return err
	}

	definitionPath, environmentPath, variablesPath := l.Paths(draft.Project, draft.Env)
	if err := os.MkdirAll(filepath.Dir(definitionPath), directoryMode); err != nil {
		return err
	}

	definition, err := yaml.Marshal(mergeDefinition(definitionPath, draft.Definition))
	if err != nil {
		return err
	}
	if err := os.WriteFile(definitionPath, definition, definitionMode); err != nil {
		return err
	}

	environment := []byte(draft.RawYAML)
	if strings.TrimSpace(draft.RawYAML) == "" {
		if environment, err = yaml.Marshal(mergeEnvironment(environmentPath, draft.Environment)); err != nil {
			return err
		}
	}
	if err := os.WriteFile(environmentPath, environment, definitionMode); err != nil {
		return err
	}

	if strings.TrimSpace(draft.RawEnv) == "" {
		os.Remove(variablesPath)
		return nil
	}
	return os.WriteFile(variablesPath, []byte(draft.RawEnv), secretMode)
}

func (l *Loader) Remove(project, env string) error {
	definitionPath, environmentPath, variablesPath := l.Paths(project, env)

	os.Remove(environmentPath)
	os.Remove(variablesPath)

	dir := filepath.Dir(definitionPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), YamlSuffix) && entry.Name() != ProjectFile {
			return nil
		}
	}
	return os.RemoveAll(dir)
}

func EnvPathFor(source string) string {
	return strings.TrimSuffix(source, YamlSuffix) + EnvSuffix
}

func mergeDefinition(path string, incoming Definition) Definition {
	existing, err := readDefinition(path)
	if err != nil {
		return incoming
	}

	existing.Git = incoming.Git
	existing.Dockerfile = incoming.Dockerfile
	existing.Context = incoming.Context
	if incoming.Name != "" {
		existing.Name = incoming.Name
	}
	if len(incoming.BuildArgs) > 0 {
		existing.BuildArgs = incoming.BuildArgs
	}
	if len(incoming.Env) > 0 {
		existing.Env = incoming.Env
	}
	return existing
}

func mergeEnvironment(path string, incoming Environment) Environment {
	existing, err := readEnvironment(path)
	if err != nil {
		return incoming
	}

	existing.Branch = incoming.Branch
	existing.Builder = incoming.Builder
	existing.Runners = incoming.Runners
	existing.Ports = incoming.Ports
	existing.Volumes = incoming.Volumes
	existing.Network = incoming.Network
	if incoming.AutoDeploy != nil {
		existing.AutoDeploy = incoming.AutoDeploy
	}
	if incoming.Restart != "" {
		existing.Restart = incoming.Restart
	}
	if incoming.Command != "" {
		existing.Command = incoming.Command
	}
	if len(incoming.Env) > 0 {
		existing.Env = incoming.Env
	}
	return existing
}

func validName(value string) error {
	if value != filepath.Base(value) || value == "." || value == ".." {
		return fmt.Errorf("%q is not a valid name", value)
	}
	for _, symbol := range value {
		switch {
		case symbol >= 'a' && symbol <= 'z', symbol >= '0' && symbol <= '9', symbol == '-', symbol == '_':
		default:
			return fmt.Errorf("%q may only contain lowercase letters, digits, - and _", value)
		}
	}
	return nil
}
