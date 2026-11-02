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

package registry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
)

func (r *Registry) GarbageCollect(ctx context.Context) (result error) {
	if !r.maintaining.CompareAndSwap(false, true) {
		return fmt.Errorf("registry maintenance is already running")
	}
	defer r.maintaining.Store(false)
	state, err := r.state(ctx)
	if err != nil {
		return err
	}
	if state != stateRunning {
		return fmt.Errorf("registry must be running before maintenance")
	}
	if err = r.Stop(ctx); err != nil {
		return err
	}
	var id string
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), startTimeout)
		defer cancel()
		if id != "" {
			if err := r.docker.Remove(cleanup, id, true); err != nil && !errors.Is(err, docker.ErrNotFound) {
				result = errors.Join(result, fmt.Errorf("stop registry garbage collection: %w", err))
				return
			}
		}
		if err := r.docker.Start(cleanup, r.containerName); err != nil {
			result = errors.Join(result, err)
			return
		}
		result = errors.Join(result, r.waitHealthy(cleanup))
	}()
	id = r.containerName + "-gc"
	created, createErr := r.docker.Create(ctx, docker.Spec{Name: r.containerName + "-gc", Image: r.options.Image, Entrypoint: []string{"registry"}, Command: []string{"garbage-collect", "/etc/docker/registry/config.yml"}, Env: map[string]string{"REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY": storageTarget}, Mounts: []docker.Mount{{Source: r.options.DataDir, Target: storageTarget}}, Labels: map[string]string{docker.LabelManaged: "true", docker.LabelRole: "registry-gc"}, Restart: "no", Network: "none"})
	if createErr != nil {
		return createErr
	}
	id = created
	if err = r.docker.Start(ctx, id); err != nil {
		return err
	}
	timer := time.NewTicker(250 * time.Millisecond)
	defer timer.Stop()
	for {
		state, err := r.docker.State(ctx, id)
		if err != nil {
			return err
		}
		if state.Status == docker.StateExited {
			if state.ExitCode != 0 {
				return fmt.Errorf("registry garbage collection exited with code %d", state.ExitCode)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
}
