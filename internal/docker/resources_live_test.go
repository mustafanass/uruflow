//go:build live

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
	"errors"
	"testing"
)

func TestLiveRemovalKeepsVolumesInUseAndIgnoresMissingResources(t *testing.T) {
	client := liveClient(t)
	ctx := context.Background()

	const network, volume = "uruflow-livetest-network", "uruflow-livetest-volume"
	holder := Spec{
		Name:    "uruflow-livetest-holder",
		Image:   liveImage,
		Command: []string{"sh", "-c", "sleep 300"},
		Labels:  ManagedLabels("livetest", "", "job-1"),
		Mounts:  []Mount{{Type: "volume", Source: volume, Target: "/data"}},
	}
	t.Cleanup(func() {
		client.Remove(context.Background(), holder.Name, true)
		client.RemoveNetwork(context.Background(), network)
		client.RemoveVolume(context.Background(), volume)
	})

	if err := client.Pull(ctx, liveImage, nil, nil); err != nil {
		t.Fatalf("pull %s: %v", liveImage, err)
	}
	if err := client.EnsureNetwork(ctx, NetworkResource{Name: network}); err != nil {
		t.Fatalf("create network: %v", err)
	}
	if err := client.EnsureVolume(ctx, VolumeResource{Name: volume}); err != nil {
		t.Fatalf("create volume: %v", err)
	}
	if _, err := client.Run(ctx, holder); err != nil {
		t.Fatalf("run a container that uses the volume: %v", err)
	}
	if err := client.RemoveVolume(ctx, volume); err == nil {
		t.Fatal("a volume still mounted by a container was removed")
	}
	if err := client.Remove(ctx, holder.Name, true); err != nil {
		t.Fatalf("remove the container: %v", err)
	}

	for pass := 0; pass < 2; pass++ {
		if err := client.RemoveNetwork(ctx, network); err != nil {
			t.Fatalf("remove network on pass %d: %v", pass, err)
		}
		if err := client.RemoveVolume(ctx, volume); err != nil {
			t.Fatalf("remove volume on pass %d: %v", pass, err)
		}
	}
	if err := client.get(ctx, "/networks/"+network, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("network survived: %v", err)
	}
	if err := client.get(ctx, "/volumes/"+volume, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("volume survived: %v", err)
	}
}
