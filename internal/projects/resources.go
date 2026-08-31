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
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

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
