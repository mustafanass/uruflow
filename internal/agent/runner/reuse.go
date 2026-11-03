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
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/mustafanass/uruflow/internal/ufp"
)

func serviceHash(request ufp.ReleaseRequest, service ufp.ServiceSpec) (string, error) {
	networks := make(map[string]ufp.NetworkResource)
	volumes := make(map[string]ufp.VolumeResource)
	for name, resource := range request.Networks {
		if resource.Name == service.Network {
			networks[name] = resource
		}
		for _, attachment := range service.Networks {
			if resource.Name == attachment.Name {
				networks[name] = resource
			}
		}
	}
	for name, resource := range request.Volumes {
		for _, attachment := range service.Volumes {
			if attachment.Type == "volume" && resource.Name == attachment.Source {
				volumes[name] = resource
			}
		}
	}
	data, err := json.Marshal(struct {
		Version  int
		Service  ufp.ServiceSpec
		Networks map[string]ufp.NetworkResource
		Volumes  map[string]ufp.VolumeResource
	}{1, service, networks, volumes})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func (r *Runner) keepContainer(ctx context.Context, request ufp.ReleaseRequest, service ufp.ServiceSpec) (string, error) {
	if service.Mode == "job" {
		return "", nil
	}
	name := ContainerName(request.Project, service.Name)
	exists, owned, err := r.docker.ContainerOwnership(ctx, name, request.Project)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	if !owned {
		return "", fmt.Errorf("container name %s is owned outside uruflow project %s", name, request.Project)
	}
	hash, err := serviceHash(request, service)
	if err != nil {
		return "", err
	}
	state, err := r.docker.State(ctx, name)
	if err != nil {
		return "", err
	}
	if !state.Ready() || state.SpecHash != hash || state.Image != service.Image {
		return "", nil
	}
	if service.Healthcheck != nil && (service.Healthcheck.Type == "http" || service.Healthcheck.Type == "tcp") {
		health := *service.Healthcheck
		health.StartPeriod, health.Retries = 0, 1
		if err := r.waitProbe(ctx, name, &health); err != nil {
			return "", nil
		}
	}
	return state.ID, nil
}
