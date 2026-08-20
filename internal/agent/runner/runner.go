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
	"fmt"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	NamePrefix     = "uruflow-"
	PreviousSuffix = "-previous"
	stopTimeout    = 15 * time.Second
	settleWindow   = 5 * time.Second
	readyTimeout   = 2 * time.Minute
	shortIDSize    = 12
)

type Runner struct {
	docker *docker.Client
	auth   func() *docker.Auth
}

type replacement struct {
	name     string
	previous string
	replaced bool
}

func New(engine *docker.Client, auth func() *docker.Auth) *Runner {
	return &Runner{docker: engine, auth: auth}
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
			r.rollback(ctx, done, log)
			return err
		}
	}

	for _, state := range done {
		if state.replaced {
			r.docker.Remove(ctx, state.previous, true)
		}
	}

	log(ufp.StreamStdout, fmt.Sprintf("%s is live (%d service(s))", request.Project, len(request.Services)))
	return nil
}

func (r *Runner) replace(ctx context.Context, request ufp.ReleaseRequest, service ufp.ServiceSpec, log ufp.LogFunc) (replacement, error) {
	name := ContainerName(request.Project, service.Name)
	state := replacement{name: name, previous: name + PreviousSuffix}

	r.docker.Remove(ctx, state.previous, true)

	if r.docker.Exists(ctx, name) {
		log(ufp.StreamStdout, "setting "+name+" aside")
		r.docker.Stop(ctx, name, stopTimeout)
		if err := r.docker.Rename(ctx, name, state.previous); err != nil {
			return state, fmt.Errorf("set aside %s: %w", name, err)
		}
		state.replaced = true
	}

	log(ufp.StreamStdout, "starting "+name)
	id, err := r.docker.Run(ctx, spec(name, request, service))
	if err != nil {
		return state, err
	}

	log(ufp.StreamStdout, "waiting for "+name+" to come up")
	if err := r.docker.WaitReady(ctx, id, settleWindow, readyTimeout); err != nil {
		r.docker.Remove(ctx, name, true)
		return state, fmt.Errorf("%s: %w", name, err)
	}

	log(ufp.StreamStdout, fmt.Sprintf("%s is ready as %s", name, shortID(id)))
	return state, nil
}

func (r *Runner) rollback(ctx context.Context, done []replacement, log ufp.LogFunc) {
	ctx = context.WithoutCancel(ctx)
	log(ufp.StreamStderr, "release failed, restoring the previous containers")

	for index := len(done) - 1; index >= 0; index-- {
		state := done[index]
		if !state.replaced {
			r.docker.Remove(ctx, state.name, true)
			continue
		}

		r.docker.Remove(ctx, state.name, true)
		if err := r.docker.Rename(ctx, state.previous, state.name); err != nil {
			log(ufp.StreamStderr, "could not restore "+state.name+": "+err.Error())
			continue
		}
		if err := r.docker.Start(ctx, state.name); err != nil {
			log(ufp.StreamStderr, "could not restart "+state.name+": "+err.Error())
			continue
		}
		log(ufp.StreamStdout, "restored "+state.name)
	}
}

func (r *Runner) Stop(ctx context.Context, project string) error {
	return r.each(ctx, project, func(name string) error {
		return r.docker.Stop(ctx, name, stopTimeout)
	})
}

func (r *Runner) Remove(ctx context.Context, project string) error {
	return r.each(ctx, project, func(name string) error {
		r.docker.Remove(ctx, name+PreviousSuffix, true)
		return r.docker.Remove(ctx, name, true)
	})
}

func (r *Runner) each(ctx context.Context, project string, action func(string) error) error {
	containers, err := r.docker.ListContainers(ctx, true)
	if err != nil {
		return err
	}

	var failure error
	found := false

	for _, container := range containers {
		if container.Project != project {
			continue
		}
		found = true
		if err := action(container.Name); err != nil {
			failure = err
		}
	}

	if !found {
		return action(ContainerName(project, ""))
	}
	return failure
}

func (r *Runner) authFor(image string) *docker.Auth {
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
