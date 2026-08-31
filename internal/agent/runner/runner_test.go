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

func TestNativeServiceSpecReachesDocker(t *testing.T) {
	container, err := spec("uruflow-api", ufp.ReleaseRequest{Project: "api", JobID: "r1"}, ufp.ServiceSpec{
		Name: "web", Image: immutableTestImage, Entrypoint: []string{"/entry"}, CommandExec: []string{"serve", "--port", "8080"},
		Ports:       []ufp.PortBinding{{HostIP: "127.0.0.1", Host: 8080, Container: 8080}},
		Networks:    []ufp.NetworkAttachment{{Name: "edge", Aliases: []string{"web"}}},
		Resources:   ufp.ResourceLimits{MemoryBytes: 256 << 20, CPUs: 1.5, PIDs: 64},
		Security:    ufp.SecuritySpec{NoNewPrivileges: true, ReadOnlyRootFS: true},
		Logging:     ufp.LogConfig{Driver: "json-file", Options: map[string]string{"max-size": "10m"}},
		Healthcheck: &ufp.HealthcheckSpec{Type: "command", Command: []string{"CMD-SHELL", "curl -f localhost:8080/health"}, Interval: time.Second, Timeout: time.Second, Retries: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if container.Ports[0].HostIP != "127.0.0.1" || container.Networks[0].Name != "edge" || container.Resources.MemoryBytes != 256<<20 || container.Healthcheck == nil {
		t.Fatalf("container = %#v", container)
	}
}

func TestReleaseCreatesDeclaredResources(t *testing.T) {
	engine := &fakeEngine{}
	runner := New(engine, nil)
	err := runner.Release(context.Background(), ufp.ReleaseRequest{Project: "api", JobID: "r1",
		Networks: map[string]ufp.NetworkResource{"edge": {Name: "urufi-edge"}},
		Volumes:  map[string]ufp.VolumeResource{"data": {Name: "urufi-data"}},
		Services: []ufp.ServiceSpec{{Name: "web", Image: immutableTestImage, Networks: []ufp.NetworkAttachment{{Name: "edge"}}, Volumes: []ufp.VolumeBinding{{Type: "volume", Source: "data", Target: "/data"}}}},
	}, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(engine.networks) != 1 || engine.runSpecs[0].Networks[0].Name != "urufi-edge" || engine.runSpecs[0].Mounts[0].Source != "urufi-data" {
		t.Fatalf("engine = %#v", engine)
	}
}

func TestJobMustCompleteBeforeReleaseContinues(t *testing.T) {
	engine := &fakeEngine{state: &docker.State{Status: docker.StateExited, ExitCode: 0}}
	runner := New(engine, nil)
	err := runner.Release(context.Background(), ufp.ReleaseRequest{Project: "api", JobID: "r1", Services: []ufp.ServiceSpec{
		{Name: "migrate", Mode: "job", Image: immutableTestImage, JobTimeout: time.Second},
		{Name: "web", Mode: "service", Image: immutableTestImage, DependsOn: []ufp.Dependency{{Service: "migrate", Condition: "completed"}}},
	}}, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(engine.runSpecs) != 2 || engine.runSpecs[0].Name != "uruflow-api-migrate" || engine.waitReadyCalls != 1 || engine.jobLogs != 1 || engine.exists["uruflow-api-migrate"] {
		t.Fatalf("job state = %#v", engine)
	}
}

func TestTimedOutJobIsRemoved(t *testing.T) {
	engine := &fakeEngine{state: &docker.State{Status: docker.StateRunning}}
	runner := New(engine, nil)
	err := runner.Release(context.Background(), ufp.ReleaseRequest{Project: "api", JobID: "r-timeout", Services: []ufp.ServiceSpec{
		{Name: "migrate", Mode: "job", Image: immutableTestImage, JobTimeout: 20 * time.Millisecond},
	}}, func(string, string) {})
	if err == nil {
		t.Fatal("timed-out job reported success")
	}
	if engine.exists["uruflow-api-migrate"] || engine.jobLogs != 1 {
		t.Fatalf("timed-out job was not logged and removed: %#v", engine)
	}
}

func TestRenameFailureRestartsTheCurrentContainer(t *testing.T) {
	engine := &fakeEngine{
		exists:    map[string]bool{"uruflow-api": true},
		renameErr: errors.New("rename failed"),
	}
	runner := New(engine, func() *docker.Auth { return nil })
	err := runner.Release(context.Background(), ufp.ReleaseRequest{
		JobID: "r1", Project: "api", Services: []ufp.ServiceSpec{{Image: immutableTestImage}},
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
		JobID: "r1", Project: "api", Services: []ufp.ServiceSpec{{Image: immutableTestImage}},
	}, func(string, string) {})
	if err == nil {
		t.Fatal("release replaced an unowned container")
	}
	if len(engine.started) != 0 || len(engine.removed) != 0 {
		t.Fatalf("unowned container was changed: started=%v removed=%v", engine.started, engine.removed)
	}
}
