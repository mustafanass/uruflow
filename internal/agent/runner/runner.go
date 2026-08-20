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
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	NamePrefix      = "uruflow-"
	PreviousSuffix  = "-previous"
	stopTimeout     = 15 * time.Second
	inspectTimeout  = 15 * time.Second
	settleWindow    = 5 * time.Second
	readyTimeout    = 2 * time.Minute
	rollbackTimeout = 2 * time.Minute
	shortIDSize     = 12
)

type Runner struct {
	docker engine
	auth   func() *docker.Auth
}

type engine interface {
	Pull(context.Context, string, *docker.Auth, func(string)) error
	ContainerOwnership(context.Context, string, string) (bool, bool, error)
	Stop(context.Context, string, time.Duration) error
	Rename(context.Context, string, string) error
	Run(context.Context, docker.Spec) (string, error)
	WaitReady(context.Context, string, time.Duration, time.Duration) error
	Remove(context.Context, string, bool) error
	Start(context.Context, string) error
	ListContainers(context.Context, bool) ([]docker.Container, error)
}

type replacement struct {
	project       string
	name          string
	previous      string
	hadCurrent    bool
	previousMoved bool
	newRunning    bool
}

func New(dockerEngine engine, auth func() *docker.Auth) *Runner {
	return &Runner{docker: dockerEngine, auth: auth}
}

func ContainerName(project, service string) string {
	if service == "" {
		return NamePrefix + project
	}
	return NamePrefix + project + "-" + service
}

func (r *Runner) Release(ctx context.Context, request ufp.ReleaseRequest, log ufp.LogFunc) error {
	for _, service := range request.Services {
		log(ufp.StreamStdout, "pulling "+service.Image)
		if err := r.pull(ctx, service.Image, log); err != nil {
			return err
		}
	}

	done := make([]replacement, 0, len(request.Services))

	for _, service := range request.Services {
		state, err := r.replace(ctx, request, service, log)
		done = append(done, state)

		if err != nil {
			restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
			restoreErr := r.rollback(restoreCtx, done, log)
			cancel()
			if restoreErr != nil {
				return errors.Join(err, fmt.Errorf("restore previous containers: %w", restoreErr))
			}
			return err
		}
	}

	for _, state := range done {
		if state.previousMoved {
			if err := r.removeOwnedIfExists(ctx, state.previous, state.project); err != nil {
				log(ufp.StreamStderr, "could not remove "+state.previous+": "+err.Error())
			}
		}
	}

	log(ufp.StreamStdout, fmt.Sprintf("%s is live (%d service(s))", request.Project, len(request.Services)))
	return nil
}

func (r *Runner) replace(ctx context.Context, request ufp.ReleaseRequest, service ufp.ServiceSpec, log ufp.LogFunc) (replacement, error) {
	name := ContainerName(request.Project, service.Name)
	state := replacement{project: request.Project, name: name, previous: name + PreviousSuffix}

	if err := r.removeOwnedIfExists(ctx, state.previous, request.Project); err != nil {
		return state, fmt.Errorf("remove stale %s: %w", state.previous, err)
	}

	exists, owned, err := r.docker.ContainerOwnership(ctx, name, request.Project)
	if err != nil {
		return state, fmt.Errorf("inspect %s: %w", name, err)
	}
	if exists && !owned {
		return state, fmt.Errorf("container name %s is owned outside uruflow project %s", name, request.Project)
	}
	state.hadCurrent = exists
	if exists {
		log(ufp.StreamStdout, "setting "+name+" aside")
		if err := r.docker.Stop(ctx, name, stopTimeout); err != nil {
			return state, fmt.Errorf("stop %s: %w", name, err)
		}
		if err := r.docker.Rename(ctx, name, state.previous); err != nil {
			failure := fmt.Errorf("set aside %s: %w", name, err)
			currentExists, _, currentErr := r.docker.ContainerOwnership(ctx, name, request.Project)
			previousExists, _, previousErr := r.docker.ContainerOwnership(ctx, state.previous, request.Project)
			if previousExists && !currentExists {
				state.previousMoved = true
			}
			return state, errors.Join(failure, currentErr, previousErr)
		}
		state.previousMoved = true
	}

	log(ufp.StreamStdout, "starting "+name)
	id, err := r.docker.Run(ctx, spec(name, request, service))
	if err != nil {
		inspectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), inspectTimeout)
		present, owned, inspectErr := r.docker.ContainerOwnership(inspectCtx, name, request.Project)
		cancel()
		state.newRunning = present && owned
		return state, errors.Join(err, inspectErr)
	}
	state.newRunning = true

	log(ufp.StreamStdout, "waiting for "+name+" to come up")
	if err := r.docker.WaitReady(ctx, id, settleWindow, readyTimeout); err != nil {
		return state, fmt.Errorf("%s: %w", name, err)
	}

	log(ufp.StreamStdout, fmt.Sprintf("%s is ready as %s", name, shortID(id)))
	return state, nil
}

func (r *Runner) rollback(ctx context.Context, done []replacement, log ufp.LogFunc) error {
	log(ufp.StreamStderr, "release failed, restoring the previous containers")
	var failures []error

	for index := len(done) - 1; index >= 0; index-- {
		state := done[index]
		if state.newRunning {
			if err := r.removeOwnedIfExists(ctx, state.name, state.project); err != nil {
				failures = append(failures, fmt.Errorf("remove replacement %s: %w", state.name, err))
				continue
			}
		}
		if !state.previousMoved {
			if state.hadCurrent {
				if err := r.docker.Start(ctx, state.name); err != nil {
					failures = append(failures, fmt.Errorf("restart %s: %w", state.name, err))
					continue
				}
				log(ufp.StreamStdout, "restored "+state.name)
			}
			continue
		}

		if err := r.docker.Rename(ctx, state.previous, state.name); err != nil {
			log(ufp.StreamStderr, "could not restore "+state.name+": "+err.Error())
			failures = append(failures, fmt.Errorf("rename %s: %w", state.previous, err))
			continue
		}
		if err := r.docker.Start(ctx, state.name); err != nil {
			log(ufp.StreamStderr, "could not restart "+state.name+": "+err.Error())
			failures = append(failures, fmt.Errorf("restart %s: %w", state.name, err))
			continue
		}
		log(ufp.StreamStdout, "restored "+state.name)
	}
	return errors.Join(failures...)
}

func (r *Runner) removeOwnedIfExists(ctx context.Context, name, project string) error {
	exists, owned, err := r.docker.ContainerOwnership(ctx, name, project)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if !owned {
		return fmt.Errorf("container %s is not owned by uruflow project %s", name, project)
	}
	return r.docker.Remove(ctx, name, true)
}

func (r *Runner) Stop(ctx context.Context, project string) error {
	containers, err := r.docker.ListContainers(ctx, true)
	if err != nil {
		return err
	}
	var failures []error
	for _, container := range containers {
		if container.Project != project || strings.HasSuffix(container.Name, PreviousSuffix) {
			continue
		}
		if err := r.docker.Stop(ctx, container.Name, stopTimeout); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (r *Runner) Remove(ctx context.Context, project string) error {
	containers, err := r.docker.ListContainers(ctx, true)
	if err != nil {
		return err
	}
	var failures []error
	found := false
	for _, container := range containers {
		if container.Project != project {
			continue
		}
		found = true
		if err := r.docker.Remove(ctx, container.Name, true); err != nil {
			failures = append(failures, err)
		}
	}
	if !found {
		return nil
	}
	return errors.Join(failures...)
}

func (r *Runner) authFor(image string) *docker.Auth {
	if r.auth == nil {
		return nil
	}
	auth := r.auth()
	if auth == nil || auth.ServerAddress == "" {
		return nil
	}
	if !strings.HasPrefix(image, auth.ServerAddress+"/") {
		return nil
	}
	return auth
}

func (r *Runner) pull(ctx context.Context, image string, log ufp.LogFunc) error {
	seen := make(map[string]bool)

	return r.docker.Pull(ctx, image, r.authFor(image), func(status string) {
		if seen[status] {
			return
		}
		seen[status] = true
		log(ufp.StreamStdout, status)
	})
}

func spec(name string, request ufp.ReleaseRequest, service ufp.ServiceSpec) docker.Spec {
	container := docker.Spec{
		Name:    name,
		Image:   service.Image,
		Env:     service.Env,
		Network: service.Network,
		Restart: service.Restart,
		Labels:  docker.ManagedLabels(request.Project, service.Name, request.JobID),
	}

	if service.Command != "" {
		container.Command = []string{"sh", "-c", service.Command}
	}
	for _, port := range service.Ports {
		container.Ports = append(container.Ports, docker.PortBinding{
			Host: port.Host, Container: port.Container, Protocol: port.Protocol,
		})
	}
	for _, volume := range service.Volumes {
		container.Mounts = append(container.Mounts, docker.Mount{
			Source: volume.Source, Target: volume.Target, ReadOnly: volume.ReadOnly,
		})
	}

	return container
}

func shortID(id string) string {
	if len(id) <= shortIDSize {
		return id
	}
	return id[:shortIDSize]
}
