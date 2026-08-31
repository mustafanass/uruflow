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
	"sort"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

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
