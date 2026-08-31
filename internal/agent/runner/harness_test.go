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
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const immutableTestImage = "repo/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type fakeEngine struct {
	exists         map[string]bool
	unowned        map[string]bool
	renameErr      error
	started        []string
	removed        []string
	endpoint       string
	runSpecs       []docker.Spec
	waitReadyCalls int
	networks       []docker.NetworkResource
	volumes        []docker.VolumeResource
	state          *docker.State
	jobLogs        int
}

func (f *fakeEngine) Pull(context.Context, string, *docker.Auth, func(string)) error { return nil }
func (f *fakeEngine) ContainerOwnership(_ context.Context, name, _ string) (bool, bool, error) {
	exists := f.exists[name]
	return exists, exists && !f.unowned[name], nil
}
func (f *fakeEngine) Stop(context.Context, string, time.Duration) error { return nil }
func (f *fakeEngine) Rename(_ context.Context, from, to string) error {
	if f.renameErr != nil {
		return f.renameErr
	}
	f.exists[from] = false
	f.exists[to] = true
	return nil
}
func (f *fakeEngine) Run(_ context.Context, spec docker.Spec) (string, error) {
	f.runSpecs = append(f.runSpecs, spec)
	if f.exists == nil {
		f.exists = make(map[string]bool)
	}
	f.exists[spec.Name] = true
	return spec.Name, nil
}
func (f *fakeEngine) WaitReady(context.Context, string, time.Duration, time.Duration) error {
	f.waitReadyCalls++
	return nil
}
func (f *fakeEngine) State(context.Context, string) (*docker.State, error) {
	if f.state != nil {
		return f.state, nil
	}
	return &docker.State{Status: docker.StateRunning}, nil
}
func (f *fakeEngine) Endpoint(context.Context, string, int) (string, error) {
	if f.endpoint == "" {
		return "127.0.0.1:1", nil
	}
	return f.endpoint, nil
}
func (f *fakeEngine) Remove(_ context.Context, name string, _ bool) error {
	f.removed = append(f.removed, name)
	f.exists[name] = false
	return nil
}
func (f *fakeEngine) Start(_ context.Context, name string) error {
	f.started = append(f.started, name)
	return nil
}
func (f *fakeEngine) ListContainers(context.Context, bool) ([]docker.Container, error) {
	return nil, nil
}
func (f *fakeEngine) EnsureNetwork(_ context.Context, resource docker.NetworkResource) error {
	f.networks = append(f.networks, resource)
	return nil
}
func (f *fakeEngine) EnsureVolume(_ context.Context, resource docker.VolumeResource) error {
	f.volumes = append(f.volumes, resource)
	return nil
}
func (f *fakeEngine) StreamLogs(_ context.Context, _ string, _ int, _ bool, onLine func(string, string)) error {
	f.jobLogs++
	onLine(ufp.StreamStdout, "migration complete")
	return nil
}
