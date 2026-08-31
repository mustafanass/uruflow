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

package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func validateReleaseRequest(request ufp.ReleaseRequest) error {
	if !models.ValidResourceName(request.Project) {
		return fmt.Errorf("invalid project name %q", request.Project)
	}
	if len(request.Services) == 0 {
		return errors.New("release contains no services")
	}
	for key, resource := range request.Networks {
		if !models.ValidResourceName(key) || !models.ValidResourceName(resource.Name) {
			return fmt.Errorf("invalid network %q", key)
		}
	}
	for key, resource := range request.Volumes {
		if !models.ValidResourceName(key) || !models.ValidResourceName(resource.Name) {
			return fmt.Errorf("invalid volume %q", key)
		}
	}

	indices := make(map[string]int, len(request.Services))
	modes := make(map[string]string, len(request.Services))
	for index, service := range request.Services {
		if service.Name == "" && len(request.Services) != 1 {
			return errors.New("every service in a multi-service release must have a name")
		}
		if service.Name != "" && !models.ValidResourceName(service.Name) {
			return fmt.Errorf("invalid service name %q", service.Name)
		}
		if _, exists := indices[service.Name]; exists {
			return fmt.Errorf("service %q is duplicated", service.Name)
		}
		indices[service.Name] = index
		mode := service.Mode
		if mode == "" {
			mode = models.ServiceModeService
		}
		if mode != models.ServiceModeService && mode != models.ServiceModeJob {
			return fmt.Errorf("service %q has invalid mode %q", service.Name, service.Mode)
		}
		modes[service.Name] = mode
		if !models.ValidDigestReference(service.Image) {
			return fmt.Errorf("service %q image is not an immutable digest", service.Name)
		}
		if !models.ValidRestartPolicy(service.Restart) {
			return fmt.Errorf("service %q has invalid restart policy %q", service.Name, service.Restart)
		}
		if service.Resources.MemoryBytes < 0 || service.Resources.CPUs < 0 || service.Resources.PIDs < 0 {
			return fmt.Errorf("service %q has invalid resource limits", service.Name)
		}
		if service.Command != "" && len(service.CommandExec) > 0 {
			return fmt.Errorf("service %q sets both shell and exec commands", service.Name)
		}
		if service.Healthcheck != nil {
			health := &models.Healthcheck{
				Type: service.Healthcheck.Type, Scheme: service.Healthcheck.Scheme,
				Path: service.Healthcheck.Path, Port: service.Healthcheck.Port,
				Interval: service.Healthcheck.Interval, Timeout: service.Healthcheck.Timeout,
				Retries: service.Healthcheck.Retries, StableFor: service.Healthcheck.StableFor,
				Command: service.Healthcheck.Command, StartPeriod: service.Healthcheck.StartPeriod,
			}
			if err := models.ValidateHealthcheck(health); err != nil {
				return fmt.Errorf("service %q: %w", service.Name, err)
			}
		}
	}

	for index, service := range request.Services {
		for _, dependency := range service.DependsOn {
			dependencyIndex, exists := indices[dependency.Service]
			if !exists {
				return fmt.Errorf("service %q depends on unknown service %q", service.Name, dependency.Service)
			}
			if dependencyIndex >= index {
				return fmt.Errorf("service %q is not ordered after dependency %q", service.Name, dependency.Service)
			}
			if dependency.Condition != models.DependencyStarted && dependency.Condition != models.DependencyHealthy && dependency.Condition != models.DependencyCompleted {
				return fmt.Errorf("service %q dependency %q has invalid condition %q", service.Name, dependency.Service, dependency.Condition)
			}
			if dependency.Condition == models.DependencyCompleted && modes[dependency.Service] != models.ServiceModeJob {
				return fmt.Errorf("service %q requires non-job %q to complete", service.Name, dependency.Service)
			}
		}
		for _, network := range service.Networks {
			if _, exists := request.Networks[network.Name]; !exists {
				return fmt.Errorf("service %q uses undeclared network %q", service.Name, network.Name)
			}
		}
		for _, volume := range service.Volumes {
			if volume.Type == "volume" {
				if _, exists := request.Volumes[volume.Source]; !exists {
					return fmt.Errorf("service %q uses undeclared volume %q", service.Name, volume.Source)
				}
			}
		}
	}
	return nil
}

func (r *Runner) ensureResources(ctx context.Context, request ufp.ReleaseRequest) error {
	if len(request.Networks) == 0 && len(request.Volumes) == 0 {
		return nil
	}
	engine, ok := r.docker.(resourceEngine)
	if !ok {
		return fmt.Errorf("docker engine cannot manage declared resources")
	}
	for _, network := range request.Networks {
		if err := engine.EnsureNetwork(ctx, docker.NetworkResource{Name: network.Name, Driver: network.Driver, External: network.External, Internal: network.Internal, Attachable: network.Attachable, Options: network.Options, Labels: network.Labels}); err != nil {
			return fmt.Errorf("ensure network %s: %w", network.Name, err)
		}
	}
	for _, volume := range request.Volumes {
		if err := engine.EnsureVolume(ctx, docker.VolumeResource{Name: volume.Name, Driver: volume.Driver, External: volume.External, Options: volume.Options, Labels: volume.Labels}); err != nil {
			return fmt.Errorf("ensure volume %s: %w", volume.Name, err)
		}
	}
	return nil
}

func resolveResourceNames(request *ufp.ReleaseRequest) {
	for index := range request.Services {
		service := &request.Services[index]
		for networkIndex := range service.Networks {
			if resource, ok := request.Networks[service.Networks[networkIndex].Name]; ok {
				service.Networks[networkIndex].Name = resource.Name
			}
		}
		for volumeIndex := range service.Volumes {
			if resource, ok := request.Volumes[service.Volumes[volumeIndex].Source]; ok {
				service.Volumes[volumeIndex].Source = resource.Name
			}
		}
	}
}
