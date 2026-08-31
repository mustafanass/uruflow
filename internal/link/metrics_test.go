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

package link

import (
	"context"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/logic"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const builtImageForMetrics = "registry/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestMetricsLandInTheStore(t *testing.T) {
	server, store, caPEM := newTestServer(t)

	store.CreateAgent(&models.Agent{ID: "a1", Name: "runner-01", Key: "k",
		Roles: []models.Role{models.RoleRunner}})

	hello := ufp.Hello{AgentID: "a1", Roles: []ufp.Role{ufp.RoleRunner}}
	conn := dialAgent(t, server, caPEM, hello, "k")
	defer conn.Close()

	go conn.Serve(context.Background(), &clientHandler{registry: make(chan ufp.RegistryConfig, 1)})
	if err := conn.SendEvent(ufp.TopicRegistryReady, ufp.Accepted{}); err != nil {
		t.Fatal(err)
	}

	metrics := ufp.Metrics{
		System:              ufp.SystemMetrics{CPUPercent: 42, MemoryPercent: 55, Uptime: 1200},
		ContainersAvailable: true,
		Containers: []ufp.ContainerStatus{
			{ID: "c1", Name: "uruflow-api", Project: "api", State: "running"},
		},
	}
	if err := conn.SendEvent(ufp.TopicMetrics, metrics); err != nil {
		t.Fatalf("send metrics: %v", err)
	}

	deadline := time.Now().Add(settleWindow)
	for time.Now().Before(deadline) {
		agent, err := store.GetAgent("a1")
		if err == nil && agent.Metrics.CPUPercent == 42 {
			containers, _ := store.ListContainersByAgent("a1")
			if len(containers) == 1 && containers[0].Project == "api" {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatal("metrics never reached the store")
}

func TestContainerSnapshotFailurePreservesStateAndMissingContainersAlert(t *testing.T) {
	server, store, _ := newTestServer(t)
	identity := &ufp.Identity{AgentID: "a1", Name: "runner-01", Roles: []ufp.Role{ufp.RoleRunner}}
	if err := store.CreateAgent(&models.Agent{ID: "a1", Name: "runner-01", Key: "k",
		Roles: []models.Role{models.RoleRunner}}); err != nil {
		t.Fatal(err)
	}
	project := models.Project{Name: "api", GitURL: "git@host:api.git", Branch: "main", Runners: []string{"a1"}}
	if err := store.SaveProject(&project); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRelease(&models.Release{
		ID: "r1", Project: "api", Image: builtImageForMetrics, Status: models.StatusSucceeded,
		Spec: project, StartedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	server.applyMetrics(identity, ufp.Metrics{
		ContainersAvailable: true,
		Containers:          []ufp.ContainerStatus{{ID: "c1", Name: "uruflow-api", Project: "api", State: "running"}},
	})
	server.applyMetrics(identity, ufp.Metrics{ContainersAvailable: false})
	containers, err := store.ListContainersByAgent("a1")
	if err != nil || len(containers) != 1 {
		t.Fatalf("containers after failed snapshot = %+v, %v", containers, err)
	}

	server.applyMetrics(identity, ufp.Metrics{ContainersAvailable: true})
	alerts, err := store.ListActiveAlerts()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, alert := range alerts {
		if alert.Type == logic.KindContainerDown && alert.Message == "Container uruflow-api is not running" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing container alert was not raised: %+v", alerts)
	}
}
