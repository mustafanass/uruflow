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
	rollbackTimeout = 2 * time.Minute
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
	State(context.Context, string) (*docker.State, error)
	Endpoint(context.Context, string, int) (string, error)
	Remove(context.Context, string, bool) error
	Start(context.Context, string) error
	ListContainers(context.Context, bool) ([]docker.Container, error)
}

type resourceEngine interface {
	EnsureNetwork(context.Context, docker.NetworkResource) error
	EnsureVolume(context.Context, docker.VolumeResource) error
}

type logEngine interface {
	StreamLogs(context.Context, string, int, bool, func(string, string)) error
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
	if err := validateReleaseRequest(request); err != nil {
		return fmt.Errorf("invalid release request: %w", err)
	}
	if err := r.ensureResources(ctx, request); err != nil {
		return err
	}
	resolveResourceNames(&request)
	for _, service := range request.Services {
		log(ufp.StreamStdout, "pulling "+service.Image)
		if err := r.pull(ctx, service.Image, log); err != nil {
			return err
		}
	}

	done := make([]replacement, 0, len(request.Services))

	for _, service := range request.Services {
		if service.Mode == "job" {
			if err := r.runJob(ctx, request, service, log); err != nil {
				restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
				restoreErr := r.rollback(restoreCtx, done, log)
				cancel()
				return errors.Join(err, restoreErr)
			}
			continue
		}
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

func (r *Runner) runJob(ctx context.Context, request ufp.ReleaseRequest, service ufp.ServiceSpec, log ufp.LogFunc) error {
	name := ContainerName(request.Project, service.Name)
	if err := r.removeOwnedIfExists(ctx, name, request.Project); err != nil {
		return fmt.Errorf("remove previous job %s: %w", name, err)
	}
	container, err := spec(name, request, service)
	if err != nil {
		return err
	}
	container.Restart = "no"
	id, err := r.docker.Run(ctx, container)
	if err != nil {
		return fmt.Errorf("start job %s: %w", name, err)
	}
	timeout := service.JobTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		state, err := r.docker.State(ctx, id)
		if err != nil {
			return errors.Join(err, r.cleanupJob(name, request.Project))
		}
		if state.Status == docker.StateExited || state.Status == docker.StateDead {
			r.streamJobLogs(ctx, id, log)
			cleanupErr := r.cleanupJob(name, request.Project)
			if state.ExitCode != 0 {
				return errors.Join(fmt.Errorf("job %s exited with code %d", name, state.ExitCode), cleanupErr)
			}
			log(ufp.StreamStdout, "job "+name+" completed")
			return cleanupErr
		}
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), r.cleanupJob(name, request.Project))
		case <-deadline.C:
			r.streamJobLogs(context.Background(), id, log)
			return errors.Join(fmt.Errorf("job %s did not complete within %s", name, timeout), r.cleanupJob(name, request.Project))
		case <-time.After(readinessPoll):
		}
	}
}

func (r *Runner) streamJobLogs(ctx context.Context, id string, log ufp.LogFunc) {
	engine, ok := r.docker.(logEngine)
	if !ok {
		return
	}
	logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := engine.StreamLogs(logCtx, id, 0, false, log); err != nil {
		log(ufp.StreamStderr, "could not read job logs: "+err.Error())
	}
}

func (r *Runner) cleanupJob(name, project string) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), inspectTimeout)
	defer cancel()
	return r.removeOwnedIfExists(cleanupCtx, name, project)
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
	containerSpec, err := spec(name, request, service)
	if err != nil {
		return state, err
	}
	id, err := r.docker.Run(ctx, containerSpec)
	if err != nil {
		inspectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), inspectTimeout)
		present, owned, inspectErr := r.docker.ContainerOwnership(inspectCtx, name, request.Project)
		cancel()
		state.newRunning = present && owned
		return state, errors.Join(err, inspectErr)
	}
	state.newRunning = true

	log(ufp.StreamStdout, "waiting for "+name+" to come up")
	if err := r.waitReady(ctx, id, service.Healthcheck); err != nil {
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

func spec(name string, request ufp.ReleaseRequest, service ufp.ServiceSpec) (docker.Spec, error) {
	labels, err := docker.ContainerLabels(service.Labels, request.Project, service.Name, request.JobID)
	if err != nil {
		return docker.Spec{}, fmt.Errorf("service %q: %w", service.Name, err)
	}
	container := docker.Spec{
		Name:       name,
		Image:      service.Image,
		Env:        service.Env,
		Network:    service.Network,
		Restart:    service.Restart,
		Labels:     labels,
		Entrypoint: service.Entrypoint,
		Resources:  docker.ResourceLimits{MemoryBytes: service.Resources.MemoryBytes, CPUs: service.Resources.CPUs, PIDs: service.Resources.PIDs},
		Security:   docker.Security{NoNewPrivileges: service.Security.NoNewPrivileges, ReadOnlyRootFS: service.Security.ReadOnlyRootFS, User: service.Security.User, CapAdd: service.Security.CapAdd, CapDrop: service.Security.CapDrop},
		Logging:    docker.LogConfig{Driver: service.Logging.Driver, Options: service.Logging.Options},
	}

	if len(service.CommandExec) > 0 {
		container.Command = service.CommandExec
	} else if service.Command != "" {
		container.Command = []string{"sh", "-c", service.Command}
	}
	for _, network := range service.Networks {
		container.Networks = append(container.Networks, docker.NetworkAttachment{Name: network.Name, Aliases: network.Aliases})
	}
	if service.Healthcheck != nil && service.Healthcheck.Type == "command" {
		container.Healthcheck = &docker.Healthcheck{Test: service.Healthcheck.Command, Interval: service.Healthcheck.Interval, Timeout: service.Healthcheck.Timeout, Retries: service.Healthcheck.Retries, StartPeriod: service.Healthcheck.StartPeriod}
	}
	for _, port := range service.Ports {
		container.Ports = append(container.Ports, docker.PortBinding{
			HostIP: port.HostIP, Host: port.Host, Container: port.Container, Protocol: port.Protocol,
		})
	}
	for _, volume := range service.Volumes {
		container.Mounts = append(container.Mounts, docker.Mount{
			Type: volume.Type, Source: volume.Source, Target: volume.Target, ReadOnly: volume.ReadOnly, CreateHostPath: volume.CreateHostPath,
		})
	}

	return container, nil
}
