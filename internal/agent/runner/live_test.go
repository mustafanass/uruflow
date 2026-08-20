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
	"os"
	"testing"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	liveGate    = "URUFLOW_DOCKER_TESTS"
	liveImage   = "alpine:3.20"
	liveProject = "livetest"
)

func liveRunner(t *testing.T) (*Runner, *docker.Client) {
	t.Helper()

	if os.Getenv(liveGate) == "" {
		t.Skip("set " + liveGate + "=1 to run tests that talk to a real docker daemon")
	}

	client, err := docker.New(docker.DefaultSocket)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}

	t.Cleanup(func() {
		name := ContainerName(liveProject, "")
		client.Remove(context.Background(), name, true)
		client.Remove(context.Background(), name+PreviousSuffix, true)
	})

	return New(client, func() *docker.Auth { return nil }), client
}

func release(project, command string) ufp.ReleaseRequest {
	return ufp.ReleaseRequest{
		JobID:   "job-" + command,
		Project: project,
		Services: []ufp.ServiceSpec{{
			Image:   liveImage,
			Command: command,
			Restart: "unless-stopped",
		}},
	}
}

func TestLiveFailedReleaseRestoresTheRunningContainer(t *testing.T) {
	runner, client := liveRunner(t)
	ctx := context.Background()
	name := ContainerName(liveProject, "")

	quiet := func(stream, line string) { t.Logf("  %s | %s", stream, line) }

	if err := runner.Release(ctx, release(liveProject, "sleep 300"), quiet); err != nil {
		t.Fatalf("first release: %v", err)
	}

	before, err := client.Inspect(ctx, name)
	if err != nil {
		t.Fatalf("inspect after the first release: %v", err)
	}
	original, _ := client.State(ctx, name)
	if original.Status != docker.StateRunning {
		t.Fatalf("first release did not leave a running container: %+v", original)
	}
	_ = before

	err = runner.Release(ctx, release(liveProject, "exit 1"), quiet)
	if err == nil {
		t.Fatal("a release of a crashing image reported success")
	}
	t.Logf("bad release rejected: %v", err)

	restored, err := client.State(ctx, name)
	if err != nil {
		t.Fatalf("the service container is gone after a failed release: %v", err)
	}
	if restored.Status != docker.StateRunning {
		t.Fatalf("the service was left in state %q after a failed release", restored.Status)
	}

	if client.Exists(ctx, name+PreviousSuffix) {
		t.Error("the set-aside container was left behind after the restore")
	}
}

func TestLiveSuccessfulReleaseReplacesAndCleansUp(t *testing.T) {
	runner, client := liveRunner(t)
	ctx := context.Background()
	name := ContainerName(liveProject, "")

	quiet := func(stream, line string) {}

	if err := runner.Release(ctx, release(liveProject, "sleep 300"), quiet); err != nil {
		t.Fatalf("first release: %v", err)
	}
	first, _ := client.Inspect(ctx, name)

	if err := runner.Release(ctx, release(liveProject, "sleep 301"), quiet); err != nil {
		t.Fatalf("second release: %v", err)
	}
	second, _ := client.Inspect(ctx, name)

	if first.Config.Labels[docker.LabelRelease] == second.Config.Labels[docker.LabelRelease] {
		t.Error("the second release did not actually replace the container")
	}
	if client.Exists(ctx, name+PreviousSuffix) {
		t.Error("the set-aside container was not cleaned up after a successful release")
	}

	state, _ := client.State(ctx, name)
	if state.Status != docker.StateRunning {
		t.Fatalf("container state after the second release = %q", state.Status)
	}
}

func multiRelease(project string, commands map[string]string) ufp.ReleaseRequest {
	request := ufp.ReleaseRequest{JobID: "job-multi", Project: project}
	for _, name := range []string{"app", "worker"} {
		request.Services = append(request.Services, ufp.ServiceSpec{
			Name:    name,
			Image:   liveImage,
			Command: commands[name],
			Restart: "unless-stopped",
		})
	}
	return request
}

func TestLiveMultiServiceRestoresEveryServiceWhenOneFails(t *testing.T) {
	runner, client := liveRunner(t)
	ctx := context.Background()
	quiet := func(stream, line string) { t.Logf("  %s | %s", stream, line) }

	appName := ContainerName(liveProject, "app")
	workerName := ContainerName(liveProject, "worker")
	t.Cleanup(func() {
		for _, n := range []string{appName, workerName, appName + PreviousSuffix, workerName + PreviousSuffix} {
			client.Remove(context.Background(), n, true)
		}
	})

	good := multiRelease(liveProject, map[string]string{"app": "sleep 300", "worker": "sleep 300"})
	if err := runner.Release(ctx, good, quiet); err != nil {
		t.Fatalf("first multi-service release: %v", err)
	}

	first, err := client.Inspect(ctx, appName)
	if err != nil {
		t.Fatalf("app not running after first release: %v", err)
	}
	firstRelease := first.Config.Labels[docker.LabelRelease]

	if state, _ := client.State(ctx, workerName); state.Status != docker.StateRunning {
		t.Fatalf("worker not running after first release: %+v", state)
	}

	bad := multiRelease(liveProject, map[string]string{"app": "sleep 300", "worker": "exit 1"})
	bad.JobID = "job-multi-bad"
	if err := runner.Release(ctx, bad, quiet); err == nil {
		t.Fatal("a release whose second service crashes reported success")
	} else {
		t.Logf("rejected: %v", err)
	}

	appState, err := client.State(ctx, appName)
	if err != nil || appState.Status != docker.StateRunning {
		t.Fatalf("app was not restored after the worker failed: %+v err=%v", appState, err)
	}
	workerState, err := client.State(ctx, workerName)
	if err != nil || workerState.Status != docker.StateRunning {
		t.Fatalf("worker was not restored: %+v err=%v", workerState, err)
	}

	restored, _ := client.Inspect(ctx, appName)
	if restored.Config.Labels[docker.LabelRelease] != firstRelease {
		t.Errorf("app is running the failed release's container, not the restored one")
	}

	for _, n := range []string{appName + PreviousSuffix, workerName + PreviousSuffix} {
		if client.Exists(ctx, n) {
			t.Errorf("%s was left behind", n)
		}
	}
}
