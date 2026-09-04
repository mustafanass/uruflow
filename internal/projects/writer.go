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

	"github.com/mustafanass/uruflow/internal/models"
	"gopkg.in/yaml.v3"
)

const (
	yamlMode      = 0o644
	secretMode    = 0o600
	directoryMode = 0o755
)

type Draft struct {
	Project     string
	Env         string
	Environment Environment
	RawYAML     string
	RawEnv      string
}

func (l *Loader) Paths(project, env string) (environment, variables string) {
	dir := filepath.Join(l.dir, project)
	return filepath.Join(dir, env+YAMLSuffix),
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

	environmentPath, variablesPath := l.Paths(draft.Project, draft.Env)

	environment := []byte(draft.RawYAML)
	var err error
	if strings.TrimSpace(draft.RawYAML) == "" {
		if environment, err = yaml.Marshal(mergeEnvironment(environmentPath, draft.Environment)); err != nil {
			return err
		}
	}
	if err := l.WriteEnvironment(draft.Project, draft.Env, environment); err != nil {
		return err
	}

	if strings.TrimSpace(draft.RawEnv) == "" {
		os.Remove(variablesPath)
		return nil
	}
	return writeAtomic(variablesPath, []byte(draft.RawEnv), secretMode)
}

func (l *Loader) WriteEnvironment(project, env string, content []byte) error {
	if err := validName(project); err != nil {
		return err
	}
	if err := validName(env); err != nil {
		return err
	}
	path, _ := l.Paths(project, env)
	if err := os.MkdirAll(filepath.Dir(path), directoryMode); err != nil {
		return err
	}
	return writeAtomic(path, content, yamlMode)
}

func (l *Loader) Create(draft Draft) error {
	l.createMu.Lock()
	defer l.createMu.Unlock()

	if draft.Project == "" || draft.Env == "" {
		return fmt.Errorf("project and environment names are required")
	}
	if err := validName(draft.Project); err != nil {
		return err
	}
	if err := validName(draft.Env); err != nil {
		return err
	}
	if err := os.MkdirAll(l.dir, directoryMode); err != nil {
		return err
	}
	destination := filepath.Join(l.dir, draft.Project)
	if _, err := os.Stat(destination); err == nil {
		environmentPath, _ := l.Paths(draft.Project, draft.Env)
		if _, environmentErr := os.Stat(environmentPath); environmentErr == nil {
			return fmt.Errorf("project environment %q already exists", draft.Project+NameSeparator+draft.Env)
		} else if !os.IsNotExist(environmentErr) {
			return environmentErr
		}
		return l.Write(draft)
	} else if !os.IsNotExist(err) {
		return err
	}

	temporaryRoot, err := os.MkdirTemp(filepath.Dir(l.dir), ".uruflow-project-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporaryRoot)
	temporaryLoader := &Loader{dir: temporaryRoot, defaults: l.defaults, resolve: l.resolve}
	if err := temporaryLoader.Write(draft); err != nil {
		return err
	}
	return os.Rename(filepath.Join(temporaryRoot, draft.Project), destination)
}

func (l *Loader) Remove(project, env string) error {
	environmentPath, variablesPath := l.Paths(project, env)

	os.Remove(environmentPath)
	os.Remove(variablesPath)

	dir := filepath.Dir(environmentPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), YAMLSuffix) {
			return nil
		}
	}
	return os.RemoveAll(dir)
}

func mergeEnvironment(path string, incoming Environment) Environment {
	existing, err := readEnvironment(path)
	if err != nil {
		return incoming
	}

	existing.Workflow = incoming.Workflow
	existing.Timeout = incoming.Timeout
	existing.Builder = incoming.Builder
	existing.Runners = incoming.Runners
	existing.Ports = incoming.Ports
	existing.Volumes = incoming.Volumes
	existing.Network = incoming.Network
	if incoming.Restart != "" {
		existing.Restart = incoming.Restart
	}
	if incoming.Command.Shell != "" || len(incoming.Command.Exec) > 0 {
		existing.Command = incoming.Command
	}
	if len(incoming.Env) > 0 {
		existing.Env = incoming.Env
	}
	if incoming.Services != nil {
		existing.Services = incoming.Services
	}
	if incoming.Networks != nil {
		existing.Networks = incoming.Networks
	}
	if incoming.VolumeResources != nil {
		existing.VolumeResources = incoming.VolumeResources
	}
	if incoming.Resources.Networks != nil || incoming.Resources.Volumes != nil {
		existing.Resources = incoming.Resources
	}
	return existing
}

func validName(value string) error {
	if !models.ValidResourceName(value) {
		return fmt.Errorf("%q must start with a lowercase letter or digit, contain only lowercase letters, digits, ., - and _, and be at most 63 characters", value)
	}
	return nil
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".uruflow-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(content); err != nil {
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
	if err := os.Chmod(temporaryPath, mode); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
