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

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestUnchangedContainerIsKeptAndItsIdentityReported(t *testing.T) {
	request := ufp.ReleaseRequest{Project: "api", JobID: "next", Services: []ufp.ServiceSpec{{Name: "rust", Image: immutableTestImage, Env: map[string]string{"TOKEN": "original"}}}}
	hash, err := serviceHash(request, request.Services[0])
	if err != nil {
		t.Fatal(err)
	}
	name := ContainerName("api", "rust")
	engine := &fakeEngine{exists: map[string]bool{name: true}, state: &docker.State{ID: "original-container", Status: docker.StateRunning, Health: docker.HealthHealthy, Image: immutableTestImage, SpecHash: hash}}
	runner := New(engine, nil)
	containers, err := runner.Release(context.Background(), request, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if containers["rust"] != "original-container" || len(engine.runSpecs) != 0 || len(engine.pulled) != 0 || len(engine.stopped) != 0 {
		t.Fatalf("unchanged container was replaced: %+v %+v", containers, engine)
	}
	request.Services[0].Env = map[string]string{"TOKEN": "rotated"}
	containers, err = runner.Release(context.Background(), request, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(engine.runSpecs) != 1 || len(engine.stopped) != 1 || containers["rust"] == "original-container" {
		t.Fatal("rotated runtime configuration was not deployed")
	}
}

func TestUnhealthyContainerIsReplacedEvenWithMatchingConfiguration(t *testing.T) {
	request := ufp.ReleaseRequest{Project: "api", JobID: "next", Services: []ufp.ServiceSpec{{Name: "rust", Image: immutableTestImage}}}
	hash, _ := serviceHash(request, request.Services[0])
	name := ContainerName("api", "rust")
	engine := &fakeEngine{exists: map[string]bool{name: true}, state: &docker.State{ID: "old", Status: docker.StateRunning, Health: docker.HealthUnhealthy, Image: immutableTestImage, SpecHash: hash}}
	if _, err := New(engine, nil).Release(context.Background(), request, func(string, string) {}); err != nil {
		t.Fatal(err)
	}
	if len(engine.runSpecs) != 1 || len(engine.pulled) != 1 {
		t.Fatal("unhealthy container was kept")
	}
}

func TestFailedReplacementLeavesUnchangedServicesRunning(t *testing.T) {
	request := ufp.ReleaseRequest{Project: "api", JobID: "next", Services: []ufp.ServiceSpec{{Name: "rust", Image: immutableTestImage}, {Name: "console", Image: immutableTestImage}}}
	hash, _ := serviceHash(request, request.Services[0])
	rust, console := ContainerName("api", "rust"), ContainerName("api", "console")
	engine := &fakeEngine{exists: map[string]bool{rust: true, console: true}, state: &docker.State{ID: "rust-container", Status: docker.StateRunning, Health: docker.HealthNone, Image: immutableTestImage, SpecHash: hash}, waitReadyErr: errors.New("not ready")}
	if _, err := New(engine, nil).Release(context.Background(), request, func(string, string) {}); err == nil {
		t.Fatal("replacement should fail")
	}
	if !engine.exists[rust] || !engine.exists[console] || len(engine.stopped) != 1 || engine.stopped[0] != console {
		t.Fatalf("unchanged service was disrupted: %+v", engine)
	}
}

func TestRuntimeHashIncludesResourcesAndIgnoresReleaseIdentity(t *testing.T) {
	request := ufp.ReleaseRequest{Project: "api", JobID: "first", Networks: map[string]ufp.NetworkResource{"data": {Name: "data", Internal: true}}, Volumes: map[string]ufp.VolumeResource{"data": {Name: "data", Driver: "local"}}}
	service := ufp.ServiceSpec{Name: "api", Image: immutableTestImage, Networks: []ufp.NetworkAttachment{{Name: "data"}}, Volumes: []ufp.VolumeBinding{{Type: "volume", Source: "data", Target: "/data"}}}
	first, _ := serviceHash(request, service)
	request.JobID = "second"
	second, _ := serviceHash(request, service)
	if first != second {
		t.Fatal("release identity caused a restart")
	}
	network := request.Networks["data"]
	network.Internal = false
	request.Networks["data"] = network
	second, _ = serviceHash(request, service)
	if first == second {
		t.Fatal("changed network settings were ignored")
	}
	container, err := spec("api", request, service)
	if err != nil || container.Labels[docker.LabelSpec] != second {
		t.Fatal("runtime fingerprint was not attached")
	}
}
