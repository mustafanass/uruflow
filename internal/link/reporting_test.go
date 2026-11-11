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

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func connectReporter(t *testing.T, server *Server, store *sqlite.Store, caPEM, id, release string) *clientHandler {
	t.Helper()

	if err := store.CreateAgent(&models.Agent{
		ID: id, Name: id, Key: id + "-key",
		Roles: []models.Role{models.RoleRunner},
	}); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	hello := ufp.Hello{AgentID: id, Hostname: id, Version: release,
		Platform: "linux/amd64", Roles: []ufp.Role{ufp.RoleRunner}}
	conn := dialAgent(t, server, caPEM, hello, id+"-key")
	t.Cleanup(func() { conn.Close() })

	client := &clientHandler{registry: make(chan ufp.RegistryConfig, 2), agents: make(chan ufp.AgentConfig, 2)}
	go conn.Serve(context.Background(), client)
	return client
}

func awaitRegistry(t *testing.T, client *clientHandler, password string) {
	t.Helper()

	select {
	case registry := <-client.registry:
		if registry.Password != password {
			t.Fatalf("registry password = %q, want %q", registry.Password, password)
		}
	case <-time.After(settleWindow):
		t.Fatalf("the agent never received the registry login %q", password)
	}
}

func awaitAgentConfig(t *testing.T, client *clientHandler, want ufp.AgentConfig) {
	t.Helper()

	select {
	case config := <-client.agents:
		if config != want {
			t.Fatalf("agent config = %+v, want %+v", config, want)
		}
	case <-time.After(settleWindow):
		t.Fatalf("the agent never received %+v", want)
	}
}

func TestAgentConfigReachesOnlyAgentsThatUnderstandIt(t *testing.T) {
	server, store, caPEM := newTestServer(t)
	server.SetAgentConfig(ufp.AgentConfig{MetricsSec: 30, ReconnectSec: 10})

	current := connectReporter(t, server, store, caPEM, "current", ufp.AgentConfigSince)
	older := connectReporter(t, server, store, caPEM, "older", "3.0.5")
	awaitRegistry(t, current, "secret")
	awaitRegistry(t, older, "secret")
	awaitAgentConfig(t, current, ufp.AgentConfig{MetricsSec: 30, ReconnectSec: 10})

	server.UpdateAgentConfig(ufp.AgentConfig{MetricsSec: 60, ReconnectSec: 2})
	awaitAgentConfig(t, current, ufp.AgentConfig{MetricsSec: 60, ReconnectSec: 2})

	server.UpdateRegistry(ufp.RegistryConfig{Host: "127.0.0.1:5000", Username: "uruflow", Password: "rotated", CACert: caPEM})
	awaitRegistry(t, current, "rotated")
	awaitRegistry(t, older, "rotated")

	select {
	case config := <-older.agents:
		t.Fatalf("an agent older than %s received %+v", ufp.AgentConfigSince, config)
	default:
	}
}
