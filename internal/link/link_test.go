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
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestAgentHandshakeRegistersAndReceivesRegistry(t *testing.T) {
	server, store, caPEM := newTestServer(t)

	if err := store.CreateAgent(&models.Agent{
		ID: "a1", Name: "builder-01", Key: "agent-key",
		Roles: []models.Role{models.RoleBuilder},
	}); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	events := &recorder{connected: make(chan *models.Agent, 1), statuses: make(chan ufp.JobStatus, 1)}
	server.Subscribe(events)

	hello := ufp.Hello{AgentID: "a1", Hostname: "box", Version: "2.0.0",
		Platform: "linux/amd64", Roles: []ufp.Role{ufp.RoleBuilder}}
	conn := dialAgent(t, server, caPEM, hello, "agent-key")
	defer conn.Close()

	client := &clientHandler{registry: make(chan ufp.RegistryConfig, 1)}
	go conn.Serve(context.Background(), client)
	if err := conn.SendEvent(ufp.TopicRegistryReady, ufp.Accepted{}); err != nil {
		t.Fatal(err)
	}

	select {
	case agent := <-events.connected:
		if agent.Name != "builder-01" || agent.Platform != "linux/amd64" {
			t.Fatalf("agent = %+v", agent)
		}
	case <-time.After(settleWindow):
		t.Fatal("no AgentConnected event")
	}

	select {
	case registry := <-client.registry:
		if registry.Host != "127.0.0.1:5000" || registry.CACert == "" {
			t.Fatalf("registry handoff = %+v", registry)
		}
	case <-time.After(settleWindow):
		t.Fatal("the agent never received the registry configuration")
	}

	if !server.Online("a1") {
		t.Fatal("server does not consider the agent online")
	}

	stored, err := store.GetAgent("a1")
	if err != nil || stored.Status != models.AgentOnline {
		t.Fatalf("stored agent = %+v err = %v", stored, err)
	}
	if !stored.HasRole(models.RoleBuilder) || stored.HasRole(models.RoleRunner) {
		t.Fatalf("enrolled roles changed: %v", stored.Roles)
	}
}

func TestAgentStaysOfflineUntilRegistryReady(t *testing.T) {
	server, store, caPEM := newTestServer(t)

	if err := store.CreateAgent(&models.Agent{
		ID: "a1", Name: "runner-01", Key: "agent-key", Status: models.AgentOnline,
		Roles: []models.Role{models.RoleRunner},
	}); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	conn := dialAgent(t, server, caPEM, ufp.Hello{
		AgentID: "a1", Roles: []ufp.Role{ufp.RoleRunner},
	}, "agent-key")
	client := &clientHandler{registry: make(chan ufp.RegistryConfig, 1)}
	done := make(chan struct{})
	go func() {
		conn.Serve(context.Background(), client)
		close(done)
	}()

	select {
	case <-client.registry:
	case <-time.After(settleWindow):
		t.Fatal("the agent never received the registry configuration")
	}

	stored, err := store.GetAgent("a1")
	if err != nil || stored.Status != models.AgentOffline {
		t.Fatalf("stored agent = %+v err = %v", stored, err)
	}
	if server.Online("a1") {
		t.Fatal("server considers an unready agent online")
	}

	conn.Close()
	select {
	case <-done:
	case <-time.After(settleWindow):
		t.Fatal("agent connection did not close")
	}
	stored, err = store.GetAgent("a1")
	if err != nil || stored.Status != models.AgentOffline {
		t.Fatalf("stored agent after disconnect = %+v err = %v", stored, err)
	}
}

func TestServerRejectsUnknownAgent(t *testing.T) {
	server, _, caPEM := newTestServer(t)

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(caPEM))

	netConn, err := tls.DialWithDialer(&net.Dialer{Timeout: settleWindow}, "tcp", server.Addr(), &tls.Config{
		RootCAs: pool, ServerName: ufp.ServerName, MinVersion: tls.VersionTLS13,
	})
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	defer netConn.Close()

	hello := ufp.Hello{AgentID: "ghost", Roles: []ufp.Role{ufp.RoleRunner}}
	if _, _, err := ufp.Dial(netConn, hello, "whatever"); err == nil {
		t.Fatal("an unregistered agent completed the handshake")
	}
}

func TestRequestToOfflineAgentFails(t *testing.T) {
	server, _, _ := newTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := server.Request(ctx, "nobody", ufp.MethodReleaseRun, ufp.ReleaseRequest{}); err == nil {
		t.Fatal("expected a request to an offline agent to fail")
	}
}
