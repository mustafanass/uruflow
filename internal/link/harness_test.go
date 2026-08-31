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
	"crypto/tls"
	"crypto/x509"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/pki"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/ufp"
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
		MinVersion: tls.VersionTLS13,
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
