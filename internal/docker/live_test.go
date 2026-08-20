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

package docker

import (
	"context"
	"os"
	"testing"
	"time"
)

const (
	liveGate  = "URUFLOW_DOCKER_TESTS"
	liveImage = "alpine:3.20"
)

func liveClient(t *testing.T) *Client {
	t.Helper()

	if os.Getenv(liveGate) == "" {
		t.Skip("set " + liveGate + "=1 to run tests that talk to a real docker daemon")
	}

	client, err := New(DefaultSocket)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	return client
}

func TestLiveReadyGateAcceptsAHealthyContainerAndRejectsACrashLoop(t *testing.T) {
	client := liveClient(t)
	ctx := context.Background()

	if err := client.Pull(ctx, liveImage, nil, nil); err != nil {
		t.Fatalf("pull %s: %v", liveImage, err)
	}

	good := Spec{
		Name:    "uruflow-livetest-good",
		Image:   liveImage,
		Command: []string{"sh", "-c", "sleep 300"},
		Labels:  ManagedLabels("livetest", "", "job-1"),
		Restart: "unless-stopped",
	}
	t.Cleanup(func() { client.Remove(context.Background(), good.Name, true) })

	id, err := client.Run(ctx, good)
	if err != nil {
		t.Fatalf("run a healthy container: %v", err)
	}
	if err := client.WaitReady(ctx, id, 2*time.Second, 30*time.Second); err != nil {
		t.Fatalf("a container that stays up was not accepted: %v", err)
	}

	bad := Spec{
		Name:    "uruflow-livetest-bad",
		Image:   liveImage,
		Command: []string{"sh", "-c", "exit 1"},
		Labels:  ManagedLabels("livetest", "", "job-2"),
		Restart: "unless-stopped",
	}
	t.Cleanup(func() { client.Remove(context.Background(), bad.Name, true) })

	badID, err := client.Run(ctx, bad)
	if err != nil {
		t.Fatalf("run a crashing container: %v", err)
	}

	started := time.Now()
	if err := client.WaitReady(ctx, badID, 2*time.Second, 30*time.Second); err == nil {
		t.Fatal("a container that exits immediately was accepted as ready")
	} else {
		t.Logf("crash-loop rejected after %s: %v", time.Since(started).Round(time.Millisecond), err)
	}
	if time.Since(started) > 20*time.Second {
		t.Errorf("the gate took %s to reject a crash loop", time.Since(started))
	}
}

func TestLiveRenameRoundTrip(t *testing.T) {
	client := liveClient(t)
	ctx := context.Background()

	if err := client.Pull(ctx, liveImage, nil, nil); err != nil {
		t.Fatalf("pull: %v", err)
	}

	spec := Spec{
		Name:    "uruflow-livetest-rename",
		Image:   liveImage,
		Command: []string{"sh", "-c", "sleep 300"},
		Labels:  ManagedLabels("livetest", "", "job-3"),
	}
	t.Cleanup(func() {
		client.Remove(context.Background(), spec.Name, true)
		client.Remove(context.Background(), spec.Name+"-previous", true)
	})

	id, err := client.Run(ctx, spec)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if err := client.Rename(ctx, spec.Name, spec.Name+"-previous"); err != nil {
		t.Fatalf("rename aside: %v", err)
	}
	if client.Exists(ctx, spec.Name) {
		t.Fatal("the original name is still taken after the rename")
	}
	if err := client.Rename(ctx, spec.Name+"-previous", spec.Name); err != nil {
		t.Fatalf("rename back: %v", err)
	}

	state, err := client.State(ctx, id)
	if err != nil || state.Status != StateRunning {
		t.Fatalf("state after the round trip = %+v err = %v", state, err)
	}
}
