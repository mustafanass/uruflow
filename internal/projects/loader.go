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
	"sync"

	"github.com/mustafanass/uruflow/internal/models"
	"gopkg.in/yaml.v3"
)

type Resolver func(name string) (*models.Agent, error)

type Loader struct {
	dir      string
	defaults string
	resolve  Resolver
	createMu sync.Mutex
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

func (l *Loader) Validate(path, content string) error {
	var environment Environment
	if err := decodeStrict([]byte(content), &environment); err != nil {
		return err
	}
	defaults, err := l.readDefaults()
	if err != nil {
		return err
	}
	var environmentValues map[string]string
	if path != "" && path != "-" {
		envName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		environmentValues, err = l.readEnvFile(filepath.Dir(path), envName)
		if err != nil {
			return err
		}
	}
	return validateEnvironment(environment, mergeEnv(defaults.Env, environment.Env, environmentValues))
}

func (l *Loader) loadProject(dir, folder string, defaults Defaults, result *Result) {
	if err := validName(folder); err != nil {
		result.Problems = append(result.Problems, Problem{
			Path: dir, Reason: err})
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
		if entry.IsDir() || !strings.HasSuffix(name, YAMLSuffix) {
			continue
		}

		found = true
		envName := strings.TrimSuffix(name, YAMLSuffix)
		path := filepath.Join(dir, name)

		project, err := l.buildProject(dir, path, folder, envName, defaults)
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

func (l *Loader) buildProject(dir, path, folder, envName string, defaults Defaults) (*models.Project, error) {
	if err := validName(envName); err != nil {
		return nil, err
	}
	environment, err := readEnvironment(path)
	if err != nil {
		return nil, err
	}
	environmentValues, err := l.readEnvFile(dir, envName)
	if err != nil {
		return nil, err
	}

	effectiveEnv := mergeEnv(defaults.Env, environment.Env, environmentValues)
	effectiveEnv, err = resolveVariables(effectiveEnv)
	if err != nil {
		return nil, fmt.Errorf("environment: %w", err)
	}
	environment, err = interpolateEnvironmentRuntime(environment, effectiveEnv)
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}
	ports, err := models.ParsePorts(environment.Ports)
	if err != nil {
		return nil, err
	}
	volumes, err := models.ParseVolumes(environment.Volumes)
	if err != nil {
		return nil, err
	}
	services, err := buildServices(environment.Services, effectiveEnv)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return nil, errors.New("services must define at least one service")
	}
	workflow := environment.Workflow
	if !models.ValidWorkflow(workflow) {
		return nil, fmt.Errorf("workflow %q is not supported", workflow)
	}
	probe := models.Project{
		Workflow: workflow,
		Services: services,
		Runners:  environment.Runners,
	}
	workflow = probe.EffectiveWorkflow()
	built := false
	for _, service := range probe.ServiceList() {
		built = built || service.Built()
	}
	if workflow == models.WorkflowDeployOnly && built {
		return nil, errors.New("deploy_only requires immutable images for every service")
	}
	if workflow != models.WorkflowDeployOnly && !built {
		return nil, fmt.Errorf("%s requires at least one source-built service", workflow)
	}
	if probe.NeedsBuilder() && environment.Builder == "" {
		return nil, errors.New("build workflows require a builder")
	}
	if probe.NeedsBuilder() {
		for _, service := range probe.ServiceList() {
			if !service.Built() {
				continue
			}
			if service.GitURL == "" || service.Branch == "" {
				return nil, fmt.Errorf("service %q requires git and branch", service.Name)
			}
		}
	}
	if !probe.NeedsBuilder() && environment.Builder != "" {
		return nil, errors.New("deploy_only must not set a builder")
	}
	if probe.NeedsRunners() && len(environment.Runners) == 0 {
		return nil, errors.New("deployment workflows require at least one runner")
	}
	if !probe.NeedsRunners() && len(environment.Runners) > 0 {
		return nil, errors.New("build_only must not set runners")
	}

	builderID := ""
	if probe.NeedsBuilder() {
		builder, err := l.agent(environment.Builder, models.RoleBuilder)
		if err != nil {
			return nil, err
		}
		builderID = builder.ID
	}
	runners := make([]string, 0, len(environment.Runners))
	for _, name := range environment.Runners {
		runner, err := l.agent(name, models.RoleRunner)
		if err != nil {
			return nil, err
		}
		runners = append(runners, runner.ID)
	}
	declaredNetworks := environment.Resources.Networks
	if len(environment.Networks) > 0 {
		if len(declaredNetworks) > 0 {
			return nil, errors.New("set resource networks under resources.networks only")
		}
		declaredNetworks = environment.Networks
	}
	declaredVolumes := environment.Resources.Volumes
	if len(environment.VolumeResources) > 0 {
		if len(declaredVolumes) > 0 {
			return nil, errors.New("set resource volumes under resources.volumes only")
		}
		declaredVolumes = environment.VolumeResources
	}
	projectName := folder + NameSeparator + envName
	if !models.ValidResourceName(projectName) {
		return nil, fmt.Errorf("project name %q is invalid after adding the environment suffix", projectName)
	}
	declaredNetworks, err = interpolateNetworks(declaredNetworks, effectiveEnv)
	if err != nil {
		return nil, fmt.Errorf("networks: %w", err)
	}
	declaredVolumes, err = interpolateVolumes(declaredVolumes, effectiveEnv)
	if err != nil {
		return nil, fmt.Errorf("volumes: %w", err)
	}
	networks, err := buildNetworks(projectName, declaredNetworks)
	if err != nil {
		return nil, err
	}
	volumesResources, err := buildVolumeResources(projectName, declaredVolumes)
	if err != nil {
		return nil, err
	}
	if err := validateServiceResources(services, networks, volumesResources); err != nil {
		return nil, err
	}

	return &models.Project{
		Name:     projectName,
		Env:      envName,
		Source:   path,
		Builder:  builderID,
		Runners:  runners,
		Workflow: workflow,
		Services: services,
		Networks: networks,
		Volumes:  volumesResources,
		Runtime: models.Runtime{
			Ports:       ports,
			Volumes:     volumes,
			Network:     environment.Network,
			Restart:     environment.Restart,
			Command:     environment.Command.Shell,
			CommandExec: environment.Command.Exec,
			Env:         effectiveEnv,
		},
	}, nil
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
