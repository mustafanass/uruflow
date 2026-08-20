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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urustack/uruflow/internal/models"
	"gopkg.in/yaml.v3"
)

type Resolver func(name string) (*models.Agent, error)

type Loader struct {
	dir      string
	defaults string
	resolve  Resolver
}

func NewLoader(configDir string, resolve Resolver) *Loader {
	return &Loader{
		dir:      filepath.Join(configDir, "projects"),
		defaults: filepath.Join(configDir, DefaultsFile),
		resolve:  resolve,
	}
}

func (l *Loader) Dir() string { return l.dir }

func (l *Loader) Load() *Result {
	result := &Result{}

	defaults, err := l.readDefaults()
	if err != nil {
		result.Problems = append(result.Problems, Problem{Path: l.defaults, Reason: err})
	}

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			result.Problems = append(result.Problems, Problem{Path: l.dir, Reason: err})
		}
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		l.loadProject(filepath.Join(l.dir, entry.Name()), entry.Name(), defaults, result)
	}

	sort.Slice(result.Projects, func(i, j int) bool {
		return result.Projects[i].Name < result.Projects[j].Name
	})
	return result
}

func (l *Loader) readDefaults() (Defaults, error) {
	var defaults Defaults

	data, err := os.ReadFile(l.defaults)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults, nil
		}
		return defaults, err
	}

	return defaults, yaml.Unmarshal(data, &defaults)
}

func (l *Loader) loadProject(dir, folder string, defaults Defaults, result *Result) {
	definitionPath := filepath.Join(dir, ProjectFile)

	definition, err := readDefinition(definitionPath)
	if err != nil {
		result.Problems = append(result.Problems, Problem{Path: definitionPath, Reason: err})
		return
	}
	if definition.Name == "" {
		definition.Name = folder
	}
	if definition.Git == "" {
		result.Problems = append(result.Problems, Problem{
			Path: definitionPath, Reason: errors.New("git is required")})
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		result.Problems = append(result.Problems, Problem{Path: dir, Reason: err})
		return
	}

	found := false
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == ProjectFile || !strings.HasSuffix(name, YamlSuffix) {
			continue
		}

		found = true
		envName := strings.TrimSuffix(name, YamlSuffix)
		path := filepath.Join(dir, name)

		project, err := l.buildProject(dir, path, envName, definition, defaults)
		if err != nil {
			result.Problems = append(result.Problems, Problem{Path: path, Reason: err})
			continue
		}
		result.Projects = append(result.Projects, *project)
	}

	if !found {
		result.Problems = append(result.Problems, Problem{
			Path: dir, Reason: errors.New("no environment files found (expected dev.yaml, prod.yaml, …)")})
	}
}

func (l *Loader) buildProject(dir, path, envName string, definition Definition, defaults Defaults) (*models.Project, error) {
	environment, err := readEnvironment(path)
	if err != nil {
		return nil, err
	}
	if environment.Branch == "" {
		return nil, errors.New("branch is required")
	}
	if environment.Builder == "" {
		return nil, errors.New("builder is required")
	}
	if len(environment.Runners) == 0 {
		return nil, errors.New("at least one runner is required")
	}

	builder, err := l.agent(environment.Builder, models.RoleBuilder)
	if err != nil {
		return nil, err
	}

	runners := make([]string, 0, len(environment.Runners))
	for _, name := range environment.Runners {
		runner, err := l.agent(name, models.RoleRunner)
		if err != nil {
			return nil, err
		}
		runners = append(runners, runner.ID)
	}

	ports, err := models.ParsePorts(environment.Ports)
	if err != nil {
		return nil, err
	}

	volumes, err := models.ParseVolumes(environment.Volumes)
	if err != nil {
		return nil, err
	}

	environmentValues, err := l.readEnvFile(dir, envName)
	if err != nil {
		return nil, err
	}

	autoDeploy := true
	if environment.AutoDeploy != nil {
		autoDeploy = *environment.AutoDeploy
	}

	services, err := buildServices(environment.Services)
	if err != nil {
		return nil, err
	}

	return &models.Project{
		Name:       definition.Name + NameSeparator + envName,
		Env:        envName,
		Source:     path,
		GitURL:     definition.Git,
		Branch:     environment.Branch,
		Dockerfile: definition.Dockerfile,
		Context:    definition.Context,
		BuildArgs:  definition.BuildArgs,
		Builder:    builder.ID,
		Runners:    runners,
		AutoDeploy: autoDeploy,
		Services:   services,
		Runtime: models.Runtime{
			Ports:   ports,
			Volumes: volumes,
			Network: environment.Network,
			Restart: environment.Restart,
			Command: environment.Command,
			Env:     mergeEnv(defaults.Env, definition.Env, environment.Env, environmentValues),
		},
	}, nil
}

func buildServices(declared map[string]Service) ([]models.Service, error) {
	if len(declared) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}
	sort.Strings(names)

	services := make([]models.Service, 0, len(names))
	for _, name := range names {
		declaration := declared[name]

		if declaration.Image != "" && declaration.Dockerfile != "" {
			return nil, fmt.Errorf("service %q sets both image and dockerfile", name)
		}

		ports, err := models.ParsePorts(declaration.Ports)
		if err != nil {
			return nil, fmt.Errorf("service %q: %w", name, err)
		}
		volumes, err := models.ParseVolumes(declaration.Volumes)
		if err != nil {
			return nil, fmt.Errorf("service %q: %w", name, err)
		}

		services = append(services, models.Service{
			Name:       name,
			Image:      declaration.Image,
			Dockerfile: declaration.Dockerfile,
			Context:    declaration.Context,
			BuildArgs:  declaration.BuildArgs,
			Command:    declaration.Command,
			Ports:      ports,
			Volumes:    volumes,
			Env:        declaration.Env,
			Network:    declaration.Network,
			Restart:    declaration.Restart,
		})
	}

	return services, nil
}

func (l *Loader) readEnvFile(dir, envName string) (map[string]string, error) {
	path := filepath.Join(dir, envName+EnvSuffix)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	values, err := ParseDotEnv(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return values, nil
}

func (l *Loader) agent(name string, role models.Role) (*models.Agent, error) {
	agent, err := l.resolve(name)
	if err != nil {
		return nil, fmt.Errorf("unknown agent %q", name)
	}
	if !agent.HasRole(role) {
		return nil, fmt.Errorf("agent %q does not carry the %s role", name, role)
	}
	return agent, nil
}

func readDefinition(path string) (Definition, error) {
	var definition Definition
	return definition, decodeFile(path, &definition)
}

func readEnvironment(path string) (Environment, error) {
	var environment Environment
	return environment, decodeFile(path, &environment)
}

func decodeFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return decodeStrict(data, target)
}

func decodeStrict(data []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(target); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func mergeEnv(layers ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, layer := range layers {
		for key, value := range layer {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}
