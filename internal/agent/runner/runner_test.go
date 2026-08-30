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
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const immutableTestImage = "repo/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

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

func TestHTTPReadinessAcceptsOnlyTwoHundreds(t *testing.T) {
	status := http.StatusNoContent
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/ready" {
			t.Errorf("path = %q", request.URL.Path)
		}
		response.WriteHeader(status)
	}))
	defer server.Close()
	engine := &fakeEngine{endpoint: server.Listener.Addr().String()}
	runner := New(engine, nil)
	check := &ufp.HealthcheckSpec{Type: "http", Scheme: "http", Path: "/ready", Port: 8080, Interval: time.Millisecond, Timeout: time.Second, Retries: 1}
	if err := runner.waitReady(context.Background(), "new", check); err != nil {
		t.Fatalf("2xx healthcheck failed: %v", err)
	}
	status = http.StatusServiceUnavailable
	if err := runner.waitReady(context.Background(), "new", check); err == nil {
		t.Fatal("503 healthcheck succeeded")
	}
}

func TestTCPReadinessSucceedsAndFailureIsBounded(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	engine := &fakeEngine{endpoint: listener.Addr().String()}
	runner := New(engine, nil)
	check := &ufp.HealthcheckSpec{Type: "tcp", Port: 8080, Interval: time.Millisecond, Timeout: 50 * time.Millisecond, Retries: 2}
	if err := runner.waitReady(context.Background(), "new", check); err != nil {
		t.Fatalf("tcp healthcheck failed: %v", err)
	}

	listener.Close()
	started := time.Now()
	if err := runner.waitReady(context.Background(), "new", check); err == nil {
		t.Fatal("closed tcp endpoint was healthy")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatalf("retry behavior was not bounded: %s", time.Since(started))
	}
}

func TestRunningReadinessRequiresStableWindow(t *testing.T) {
	engine := &fakeEngine{}
	runner := New(engine, nil)
	started := time.Now()
	if err := runner.waitReady(context.Background(), "new", &ufp.HealthcheckSpec{Type: "running", StableFor: 120 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 120*time.Millisecond {
		t.Fatalf("stable window ended early: %s", time.Since(started))
	}
}

func TestRunningReadinessHonorsCancellation(t *testing.T) {
	engine := &fakeEngine{}
	runner := New(engine, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := runner.waitReady(ctx, "new", &ufp.HealthcheckSpec{Type: "running", StableFor: time.Minute}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if time.Since(started) > 300*time.Millisecond {
		t.Fatalf("cancellation was not bounded: %s", time.Since(started))
	}
}

func TestAbsentHealthcheckUsesExistingDockerReadiness(t *testing.T) {
	engine := &fakeEngine{}
	runner := New(engine, nil)
	if err := runner.waitReady(context.Background(), "new", nil); err != nil {
		t.Fatal(err)
	}
	if engine.waitReadyCalls != 1 {
		t.Fatalf("WaitReady calls = %d", engine.waitReadyCalls)
	}
}

func TestReadinessFailureRestoresAllPreviousContainers(t *testing.T) {
	engine := &fakeEngine{
		exists:   map[string]bool{"uruflow-api-app": true, "uruflow-api-worker": true},
		endpoint: "127.0.0.1:1",
	}
	runner := New(engine, nil)
	request := ufp.ReleaseRequest{JobID: "r2", Project: "api", Services: []ufp.ServiceSpec{
		{Name: "app", Image: immutableTestImage, Labels: map[string]string{"traefik.enable": "true"}},
		{Name: "worker", Image: immutableTestImage, Healthcheck: &ufp.HealthcheckSpec{Type: "tcp", Port: 1, Interval: time.Millisecond, Timeout: 10 * time.Millisecond, Retries: 1}},
	}}
	if err := runner.Release(context.Background(), request, func(string, string) {}); err == nil {
		t.Fatal("failed readiness reported success")
	}
	if !engine.exists["uruflow-api-app"] || !engine.exists["uruflow-api-worker"] {
		t.Fatalf("previous containers were not restored: %v", engine.exists)
	}
	if len(engine.runSpecs) != 2 || engine.runSpecs[0].Labels["traefik.enable"] != "true" || engine.runSpecs[0].Labels[docker.LabelProject] != "api" {
		t.Fatalf("created specs = %+v", engine.runSpecs)
	}
}

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
