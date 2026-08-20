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
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestCredentialsOnlyGoToTheUruflowRegistry(t *testing.T) {
	runner := New(nil, func() *docker.Auth {
		return &docker.Auth{Username: "uruflow", Password: "secret", ServerAddress: "reg.internal:5000"}
	})

	cases := map[string]bool{
		"reg.internal:5000/uruflow/api:abc": true,
		"reg.internal:5000/uruflow/api":     true,
		"redis:7-alpine":                    false,
		"docker.io/library/redis:7":         false,
		"ghcr.io/acme/thing:1":              false,
		"reg.internal:5001/uruflow/api:abc": false,
		"evil-reg.internal:5000/x/y":        false,
	}

	for image, want := range cases {
		got := runner.authFor(image) != nil
		if got != want {
			t.Errorf("%s: credentials attached = %v, want %v", image, got, want)
		}
	}
}

func TestNoCredentialsWhenTheRegistryIsUnconfigured(t *testing.T) {
	runner := New(nil, func() *docker.Auth { return nil })

	if runner.authFor("reg.internal:5000/uruflow/api") != nil {
		t.Error("credentials were invented without a configured registry")
	}
}

func TestContainerNaming(t *testing.T) {
	if name := ContainerName("api-prod", ""); name != "uruflow-api-prod" {
		t.Errorf("single-service name = %q", name)
	}
	if name := ContainerName("api-prod", "worker"); name != "uruflow-api-prod-worker" {
		t.Errorf("multi-service name = %q", name)
	}
}

func TestRenameFailureRestartsTheCurrentContainer(t *testing.T) {
	engine := &fakeEngine{
		exists:    map[string]bool{"uruflow-api": true},
		renameErr: errors.New("rename failed"),
	}
	runner := New(engine, func() *docker.Auth { return nil })
	err := runner.Release(context.Background(), ufp.ReleaseRequest{
		JobID: "r1", Project: "api", Services: []ufp.ServiceSpec{{Image: "repo/api@sha256:" + string(make([]byte, 64))}},
	}, func(string, string) {})
	if err == nil {
		t.Fatal("release succeeded after rename failed")
	}
	if len(engine.started) != 1 || engine.started[0] != "uruflow-api" {
		t.Fatalf("started = %v", engine.started)
	}
	for _, removed := range engine.removed {
		if removed == "uruflow-api" {
			t.Fatal("the current container was removed")
		}
	}
}

func TestReleaseDoesNotTouchAnUnownedNameCollision(t *testing.T) {
	engine := &fakeEngine{
		exists:  map[string]bool{"uruflow-api": true},
		unowned: map[string]bool{"uruflow-api": true},
	}
	runner := New(engine, func() *docker.Auth { return nil })
	err := runner.Release(context.Background(), ufp.ReleaseRequest{
		JobID: "r1", Project: "api", Services: []ufp.ServiceSpec{{Image: "repo/api@sha256:digest"}},
	}, func(string, string) {})
	if err == nil {
		t.Fatal("release replaced an unowned container")
	}
	if len(engine.started) != 0 || len(engine.removed) != 0 {
		t.Fatalf("unowned container was changed: started=%v removed=%v", engine.started, engine.removed)
	}
}

type fakeEngine struct {
	exists    map[string]bool
	unowned   map[string]bool
	renameErr error
	started   []string
	removed   []string
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
func (f *fakeEngine) Run(context.Context, docker.Spec) (string, error) { return "new", nil }
func (f *fakeEngine) WaitReady(context.Context, string, time.Duration, time.Duration) error {
	return nil
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
