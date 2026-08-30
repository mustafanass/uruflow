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
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
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
	if err := validName(definition.Name); err != nil {
		result.Problems = append(result.Problems, Problem{
			Path: definitionPath, Reason: err})
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
		if entry.IsDir() || name == ProjectFile || !strings.HasSuffix(name, YAMLSuffix) {
			continue
		}

		found = true
		envName := strings.TrimSuffix(name, YAMLSuffix)
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
	if err := validName(envName); err != nil {
		return nil, err
	}
	if err := validateBuildPath("dockerfile", definition.Dockerfile); err != nil {
		return nil, err
	}
	if err := validateBuildPath("context", definition.Context); err != nil {
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

	autoDeploy := true
	if environment.AutoDeploy != nil {
		autoDeploy = *environment.AutoDeploy
	}

	effectiveEnv := mergeEnv(defaults.Env, definition.Env, environment.Env, environmentValues)
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
	workflow := environment.Workflow
	if !models.ValidWorkflow(workflow) {
		return nil, fmt.Errorf("workflow %q is not supported", workflow)
	}
	// Keep legacy single-service projects inferable. Their build definition lives
	// in project.yaml rather than the environment's services map.
	probe := models.Project{
		Workflow:   workflow,
		Dockerfile: definition.Dockerfile,
		Services:   services,
		Runners:    environment.Runners,
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
	if probe.NeedsBuilder() && (definition.Git == "" || environment.Branch == "" || environment.Builder == "") {
		return nil, errors.New("build workflows require git, branch, and builder")
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
	projectName := definition.Name + NameSeparator + envName
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
		Name:       projectName,
		Env:        envName,
		Source:     path,
		GitURL:     definition.Git,
		Branch:     environment.Branch,
		Dockerfile: definition.Dockerfile,
		Context:    definition.Context,
		BuildArgs:  definition.BuildArgs,
		Builder:    builderID,
		Runners:    runners,
		AutoDeploy: autoDeploy,
		Workflow:   workflow,
		Services:   services,
		Networks:   networks,
		Volumes:    volumesResources,
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

func buildServices(declared map[string]Service, variableSets ...map[string]string) ([]models.Service, error) {
	if len(declared) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}
	sort.Strings(names)

	services := make([]models.Service, 0, len(names))
	variables := map[string]string{}
	if len(variableSets) > 0 && variableSets[0] != nil {
		variables = variableSets[0]
	}
	for _, name := range names {
		declaration := declared[name]
		declaration, err := interpolateService(declaration, variables)
		if err != nil {
			return nil, fmt.Errorf("service %q: %w", name, err)
		}
		if err := validName(name); err != nil {
			return nil, fmt.Errorf("service name: %w", err)
		}

		if declaration.Image != "" && declaration.Dockerfile != "" {
			return nil, fmt.Errorf("service %q sets both image and dockerfile", name)
		}
		if declaration.Image != "" && !models.ValidDigestReference(declaration.Image) {
			return nil, fmt.Errorf("service %q image must use repository@sha256:digest", name)
		}
		if err := validateBuildPath("service "+name+" dockerfile", declaration.Dockerfile); err != nil {
			return nil, err
		}
		if err := validateBuildPath("service "+name+" context", declaration.Context); err != nil {
			return nil, err
		}

		ports, err := models.ParsePorts(declaration.Ports)
		if err != nil {
			return nil, fmt.Errorf("service %q: %w", name, err)
		}
		volumes, err := models.ParseVolumes(declaration.Volumes)
		if err != nil {
			return nil, fmt.Errorf("service %q: %w", name, err)
		}
		for _, mount := range declaration.Mounts {
			if mount.Type != "bind" && mount.Type != "volume" && mount.Type != "tmpfs" {
				return nil, fmt.Errorf("service %q mount type %q is not supported", name, mount.Type)
			}
			if mount.Target == "" || (mount.Type != "tmpfs" && mount.Source == "") {
				return nil, fmt.Errorf("service %q mount source and target are required", name)
			}
			if mount.Type == "tmpfs" && mount.Source != "" {
				return nil, fmt.Errorf("service %q tmpfs mount must not set source", name)
			}
			if mount.Type != "bind" && mount.Bind.CreateHostPath {
				return nil, fmt.Errorf("service %q create_host_path is only valid for bind mounts", name)
			}
			volumes = append(volumes, models.Volume{Type: mount.Type, Source: mount.Source, Target: mount.Target, ReadOnly: mount.ReadOnly, CreateHostPath: mount.Bind.CreateHostPath})
		}
		healthcheck, err := buildHealthcheck(name, declaration.Healthcheck)
		if err != nil {
			return nil, err
		}
		if err := validateLabels(name, declaration.Labels); err != nil {
			return nil, err
		}
		if declaration.Network != "" && len(declaration.Networks) > 0 {
			return nil, fmt.Errorf("service %q sets both network and networks", name)
		}
		networks := make([]models.NetworkAttachment, 0, len(declaration.Networks))
		networkNames := make([]string, 0, len(declaration.Networks))
		for network := range declaration.Networks {
			networkNames = append(networkNames, network)
		}
		sort.Strings(networkNames)
		for _, network := range networkNames {
			if err := validName(network); err != nil {
				return nil, fmt.Errorf("service %q network: %w", name, err)
			}
			networks = append(networks, models.NetworkAttachment{Name: network, Aliases: declaration.Networks[network].Aliases})
		}
		dependencies := make([]models.Dependency, 0, len(declaration.DependsOn))
		dependencyNames := make([]string, 0, len(declaration.DependsOn))
		for dependency := range declaration.DependsOn {
			dependencyNames = append(dependencyNames, dependency)
		}
		sort.Strings(dependencyNames)
		for _, dependency := range dependencyNames {
			condition := declaration.DependsOn[dependency]
			if condition == "" {
				condition = models.DependencyStarted
			}
			if condition != models.DependencyStarted && condition != models.DependencyHealthy && condition != models.DependencyCompleted {
				return nil, fmt.Errorf("service %q dependency %q has unknown condition %q", name, dependency, condition)
			}
			dependencies = append(dependencies, models.Dependency{Service: dependency, Condition: condition})
		}
		mode := declaration.Mode
		if mode == "" {
			mode = models.ServiceModeService
		}
		if mode != models.ServiceModeService && mode != models.ServiceModeJob {
			return nil, fmt.Errorf("service %q mode must be service or job", name)
		}
		if !models.ValidRestartPolicy(declaration.Restart) {
			return nil, fmt.Errorf("service %q has invalid restart policy %q", name, declaration.Restart)
		}
		memory, err := models.ParseMemory(declaration.Resources.Memory)
		if err != nil {
			return nil, fmt.Errorf("service %q resources: %w", name, err)
		}
		if declaration.Resources.CPUs < 0 || declaration.Resources.PIDs < 0 {
			return nil, fmt.Errorf("service %q resources must not be negative", name)
		}
		var jobTimeout time.Duration
		if declaration.Timeout != "" {
			jobTimeout, err = positiveDuration(fmt.Sprintf("service %q timeout", name), declaration.Timeout)
			if err != nil {
				return nil, err
			}
		}

		services = append(services, models.Service{
			Name:        name,
			Image:       declaration.Image,
			Dockerfile:  declaration.Dockerfile,
			Context:     declaration.Context,
			BuildArgs:   declaration.BuildArgs,
			GitURL:      declaration.Git,
			Branch:      declaration.Branch,
			Entrypoint:  declaration.Entrypoint,
			Command:     declaration.Command.Shell,
			CommandExec: declaration.Command.Exec,
			Ports:       ports,
			Volumes:     volumes,
			Env:         declaration.Env,
			Network:     declaration.Network,
			Networks:    networks,
			Restart:     declaration.Restart,
			Mode:        mode,
			DependsOn:   dependencies,
			Resources:   models.ResourceLimits{MemoryBytes: memory, CPUs: declaration.Resources.CPUs, PIDs: declaration.Resources.PIDs},
			Security:    models.Security{NoNewPrivileges: declaration.Security.NoNewPrivileges, ReadOnlyRootFS: declaration.Security.ReadOnlyRootFS, User: declaration.Security.User, CapAdd: declaration.Security.CapAdd, CapDrop: declaration.Security.CapDrop},
			Logging:     models.LogConfig{Driver: declaration.Logging.Driver, Options: declaration.Logging.Options},
			Job:         models.Job{Timeout: jobTimeout},
			Healthcheck: healthcheck,
			Labels:      declaration.Labels,
		})
	}

	return services, nil
}

func validateServiceResources(services []models.Service, networks map[string]models.NetworkResource, volumes map[string]models.VolumeResource) error {
	for _, service := range services {
		for _, attachment := range service.Networks {
			if _, exists := networks[attachment.Name]; !exists {
				return fmt.Errorf("service %q uses undeclared network %q", service.Name, attachment.Name)
			}
			for _, alias := range attachment.Aliases {
				if alias == "" || strings.TrimSpace(alias) != alias || strings.ContainsAny(alias, " \t\r\n") {
					return fmt.Errorf("service %q network %q has invalid alias %q", service.Name, attachment.Name, alias)
				}
			}
		}
		for _, mount := range service.Volumes {
			if mount.Type != "volume" {
				continue
			}
			if _, exists := volumes[mount.Source]; !exists {
				return fmt.Errorf("service %q uses undeclared volume %q", service.Name, mount.Source)
			}
		}
	}
	return nil
}

const (
	defaultHealthInterval = 5 * time.Second
	defaultHealthTimeout  = 3 * time.Second
	defaultHealthRetries  = 10
)

func buildHealthcheck(service string, declaration *Healthcheck) (*models.Healthcheck, error) {
	if declaration == nil {
		return nil, nil
	}

	field := func(name string) string { return fmt.Sprintf("service %q healthcheck.%s", service, name) }
	healthcheck := &models.Healthcheck{Type: declaration.Type}
	switch declaration.Type {
	case "http", "tcp", "command":
		if declaration.Type != "command" && (declaration.Port < 1 || declaration.Port > 65535) {
			return nil, fmt.Errorf("%s must be between 1 and 65535", field("port"))
		}
		healthcheck.Port = declaration.Port
		healthcheck.Interval = defaultHealthInterval
		healthcheck.Timeout = defaultHealthTimeout
		healthcheck.Retries = defaultHealthRetries
		var err error
		if declaration.StartPeriod != "" {
			healthcheck.StartPeriod, err = positiveDuration(field("start_period"), declaration.StartPeriod)
			if err != nil {
				return nil, err
			}
		}

		if declaration.Interval != "" {
			healthcheck.Interval, err = positiveDuration(field("interval"), declaration.Interval)
			if err != nil {
				return nil, err
			}
		}
		if declaration.Timeout != "" {
			healthcheck.Timeout, err = positiveDuration(field("timeout"), declaration.Timeout)
			if err != nil {
				return nil, err
			}
		}
		if declaration.Retries != nil {
			if *declaration.Retries <= 0 {
				return nil, fmt.Errorf("%s must be positive", field("retries"))
			}
			healthcheck.Retries = *declaration.Retries
		}

		if declaration.Type == "command" {
			if declaration.Port != 0 || declaration.Path != "" || declaration.Scheme != "" || declaration.StableFor != "" {
				return nil, fmt.Errorf("service %q healthcheck contains fields that are not valid for command", service)
			}
			if declaration.Command.Shell != "" {
				healthcheck.Command = []string{"CMD-SHELL", declaration.Command.Shell}
				healthcheck.Shell = true
			} else if len(declaration.Command.Exec) > 0 {
				healthcheck.Command = append([]string{"CMD"}, declaration.Command.Exec...)
			} else {
				return nil, fmt.Errorf("%s is required", field("command"))
			}
			return healthcheck, nil
		}

		if declaration.Type == "tcp" {
			if declaration.Path != "" || declaration.Scheme != "" || declaration.StableFor != "" {
				return nil, fmt.Errorf("service %q healthcheck contains fields that are not valid for tcp", service)
			}
			return healthcheck, nil
		}

		if declaration.Path == "" {
			return nil, fmt.Errorf("%s is required", field("path"))
		}
		parsed, err := url.ParseRequestURI(declaration.Path)
		if err != nil || !strings.HasPrefix(declaration.Path, "/") || parsed.IsAbs() || parsed.Host != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("%s must be a valid absolute request path", field("path"))
		}
		healthcheck.Path = declaration.Path
		healthcheck.Scheme = declaration.Scheme
		if healthcheck.Scheme == "" {
			healthcheck.Scheme = "http"
		}
		if healthcheck.Scheme != "http" && healthcheck.Scheme != "https" {
			return nil, fmt.Errorf("%s must be http or https", field("scheme"))
		}
		if declaration.StableFor != "" {
			return nil, fmt.Errorf("%s is only valid for running healthchecks", field("stable_for"))
		}
		return healthcheck, nil

	case "running":
		if declaration.StableFor == "" {
			return nil, fmt.Errorf("%s is required", field("stable_for"))
		}
		stableFor, err := positiveDuration(field("stable_for"), declaration.StableFor)
		if err != nil {
			return nil, err
		}
		if declaration.Port != 0 || declaration.Path != "" || declaration.Scheme != "" || declaration.Interval != "" || declaration.Timeout != "" || declaration.Retries != nil {
			return nil, fmt.Errorf("service %q healthcheck contains fields that are not valid for running", service)
		}
		healthcheck.StableFor = stableFor
		return healthcheck, nil

	default:
		if declaration.Type == "" {
			return nil, fmt.Errorf("%s is required", field("type"))
		}
		return nil, fmt.Errorf("%s %q is not supported", field("type"), declaration.Type)
	}
}

func buildNetworks(project string, declared map[string]Network) (map[string]models.NetworkResource, error) {
	if len(declared) == 0 {
		return nil, nil
	}
	resources := make(map[string]models.NetworkResource, len(declared))
	for key, declaration := range declared {
		if err := validName(key); err != nil {
			return nil, fmt.Errorf("network: %w", err)
		}
		name := declaration.Name
		if name == "" {
			name = key
			if project != "" {
				name = project + NameSeparator + key
			}
		}
		if !models.ValidResourceName(name) {
			return nil, fmt.Errorf("network %q has invalid Docker name %q", key, name)
		}
		resources[key] = models.NetworkResource{Name: name, Driver: declaration.Driver, External: declaration.External, Internal: declaration.Internal, Attachable: declaration.Attachable, Options: declaration.Options, Labels: declaration.Labels}
	}
	return resources, nil
}

func buildVolumeResources(project string, declared map[string]VolumeDefinition) (map[string]models.VolumeResource, error) {
	if len(declared) == 0 {
		return nil, nil
	}
	resources := make(map[string]models.VolumeResource, len(declared))
	for key, declaration := range declared {
		if err := validName(key); err != nil {
			return nil, fmt.Errorf("volume: %w", err)
		}
		name := declaration.Name
		if name == "" {
			name = key
			if project != "" {
				name = project + NameSeparator + key
			}
		}
		if !models.ValidResourceName(name) {
			return nil, fmt.Errorf("volume %q has invalid Docker name %q", key, name)
		}
		resources[key] = models.VolumeResource{Name: name, Driver: declaration.Driver, External: declaration.External, Options: declaration.Options, Labels: declaration.Labels}
	}
	return resources, nil
}

func positiveDuration(field, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", field, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be positive", field)
	}
	return duration, nil
}

func validateLabels(service string, labels map[string]string) error {
	if err := models.ValidateLabels(labels); err != nil {
		return fmt.Errorf("service %q: %w", service, err)
	}
	return nil
}

func validateBuildPath(label, value string) error {
	if !models.ValidSourcePath(value) {
		return fmt.Errorf("%s %q escapes the source directory", label, value)
	}
	return nil
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
