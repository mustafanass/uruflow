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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/urustack/uruflow/internal/config"
	"github.com/urustack/uruflow/internal/models"
	"github.com/urustack/uruflow/internal/pki"
	"github.com/urustack/uruflow/internal/storage/sqlite"
	"github.com/urustack/uruflow/internal/ufp"
)

const settleWindow = 3 * time.Second

type recorder struct {
	connected chan *models.Agent
	statuses  chan ufp.JobStatus
}

func (r *recorder) AgentConnected(agent *models.Agent)    { r.connected <- agent }
func (r *recorder) AgentDisconnected(agentID string)      {}
func (r *recorder) JobLog(string, ufp.JobLog)             {}
func (r *recorder) JobStatus(_ string, s ufp.JobStatus)   { r.statuses <- s }
func (r *recorder) ContainerLog(string, ufp.ContainerLog) {}

type clientHandler struct {
	registry chan ufp.RegistryConfig
}

func (c *clientHandler) HandleRequest(*ufp.Request) (any, error) { return ufp.Accepted{}, nil }

func (c *clientHandler) HandleEvent(event *ufp.Event) error {
	if event.Topic == ufp.TopicRegistryConfig {
		var payload ufp.RegistryConfig
		if event.Decode(&payload) == nil {
			c.registry <- payload
		}
	}
	return nil
}

func newTestServer(t *testing.T) (*Server, *sqlite.Store, string) {
	t.Helper()

	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.UFPPort = 0
	cfg.Server.DataDir = dir
	cfg.Server.Advertise = "127.0.0.1"
	cfg.Registry.Host = "127.0.0.1"

	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}

	authority, err := pki.LoadOrCreateCA(cfg.CACertPath(), cfg.CAKeyPath())
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	if err := authority.EnsureLeaf(pki.Material{
		CertPath: cfg.ServerCertPath(),
		KeyPath:  cfg.ServerKeyPath(),
		Names:    []string{ufp.ServerName, "127.0.0.1"},
	}); err != nil {
		t.Fatalf("issue server certificate: %v", err)
	}

	caPEM, err := authority.CertificatePEM()
	if err != nil {
		t.Fatalf("read CA: %v", err)
	}

	store, err := sqlite.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	server := NewServer(cfg, store)
	server.SetRegistry(ufp.RegistryConfig{
		Host: "127.0.0.1:5000", Username: "uruflow", Password: "secret", CACert: caPEM,
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start link: %v", err)
	}
	t.Cleanup(func() { server.Stop() })

	return server, store, caPEM
}

func dialAgent(t *testing.T, server *Server, caPEM string, hello ufp.Hello, key string) *ufp.Conn {
	t.Helper()

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caPEM)) {
		t.Fatal("CA is not usable")
	}

	netConn, err := tls.DialWithDialer(&net.Dialer{Timeout: settleWindow}, "tcp", server.Addr(), &tls.Config{
		RootCAs:    pool,
		ServerName: ufp.ServerName,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}

	conn, _, err := ufp.Dial(netConn, hello, key)
	if err != nil {
		netConn.Close()
		t.Fatalf("handshake: %v", err)
	}
	return conn
}

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
		Platform: "linux/amd64", Roles: []ufp.Role{ufp.RoleBuilder, ufp.RoleRunner}}
	conn := dialAgent(t, server, caPEM, hello, "agent-key")
	defer conn.Close()

	client := &clientHandler{registry: make(chan ufp.RegistryConfig, 1)}
	go conn.Serve(context.Background(), client)

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
	if !stored.HasRole(models.RoleRunner) {
		t.Fatalf("roles from hello were not persisted: %v", stored.Roles)
	}
}

func TestServerRejectsUnknownAgent(t *testing.T) {
	server, _, caPEM := newTestServer(t)

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(caPEM))

	netConn, err := tls.DialWithDialer(&net.Dialer{Timeout: settleWindow}, "tcp", server.Addr(), &tls.Config{
		RootCAs: pool, ServerName: ufp.ServerName, MinVersion: tls.VersionTLS12,
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

func TestMetricsLandInTheStore(t *testing.T) {
	server, store, caPEM := newTestServer(t)

	store.CreateAgent(&models.Agent{ID: "a1", Name: "runner-01", Key: "k",
		Roles: []models.Role{models.RoleRunner}})

	hello := ufp.Hello{AgentID: "a1", Roles: []ufp.Role{ufp.RoleRunner}}
	conn := dialAgent(t, server, caPEM, hello, "k")
	defer conn.Close()

	go conn.Serve(context.Background(), &clientHandler{registry: make(chan ufp.RegistryConfig, 1)})

	metrics := ufp.Metrics{
		System: ufp.SystemMetrics{CPUPercent: 42, MemoryPercent: 55, Uptime: 1200},
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

func TestRequestToOfflineAgentFails(t *testing.T) {
	server, _, _ := newTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := server.Request(ctx, "nobody", ufp.MethodReleaseRun, ufp.ReleaseRequest{}); err == nil {
		t.Fatal("expected a request to an offline agent to fail")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
