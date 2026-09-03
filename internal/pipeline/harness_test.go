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

package pipeline

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/link"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/pki"
	"github.com/mustafanass/uruflow/internal/registry"
	"github.com/mustafanass/uruflow/internal/secrets"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	settleWindow  = 5 * time.Second
	builtDigest   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	builtImage    = "127.0.0.1:5000/uruflow/api@" + builtDigest
	prebuiltImage = "redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	builtCommit   = "0123456789abcdef0123456789abcdef01234567"
)

type fakeAgent struct {
	conn          *ufp.Conn
	failBuild     bool
	failRun       bool
	holdBuild     chan struct{}
	holdOperation chan struct{}
	builds        chan ufp.BuildRequest
	releases      chan ufp.ReleaseRequest
	operations    chan string
}

func (a *fakeAgent) HandleRequest(request *ufp.Request) (any, error) {
	switch request.Method {
	case ufp.MethodBuildRun:
		var payload ufp.BuildRequest
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		a.builds <- payload
		go a.finishBuild(payload)
		return ufp.Accepted{JobID: payload.JobID}, nil
	case ufp.MethodReleaseRun:
		var payload ufp.ReleaseRequest
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		a.releases <- payload
		go a.finishRelease(payload)
		return ufp.Accepted{JobID: payload.JobID}, nil
	case ufp.MethodReleaseStop, ufp.MethodReleaseRemove:
		a.operations <- request.Method
		if a.holdOperation != nil {
			<-a.holdOperation
		}
		return ufp.Accepted{}, nil
	}
	return ufp.Accepted{}, nil
}

func (a *fakeAgent) HandleEvent(*ufp.Event) error { return nil }

func (a *fakeAgent) finishBuild(request ufp.BuildRequest) {
	if a.holdBuild != nil {
		<-a.holdBuild
	}
	a.conn.SendEvent(ufp.TopicJobLog, ufp.JobLog{
		JobID: request.JobID, Stage: ufp.StageBuild,
		Stream: ufp.StreamStdout, Line: "building " + request.Project,
	})

	images := make(map[string]string, len(request.Targets))
	commits := make(map[string]string, len(request.Targets))
	primary := ""
	for _, target := range request.Targets {
		images[target.Service] = target.Image + "@" + builtDigest
		commits[target.Service] = builtCommit
		if primary == "" {
			primary = images[target.Service]
		}
	}

	status := ufp.JobStatus{
		JobID: request.JobID, Stage: ufp.StageBuild,
		Status: ufp.StatusSuccess, Image: primary, Images: images,
		Commit: builtCommit, Commits: commits, Digest: builtDigest,
	}
	if a.failBuild {
		status = ufp.JobStatus{JobID: request.JobID, Stage: ufp.StageBuild,
			Status: ufp.StatusFailed, Message: "compile error"}
	}
	a.conn.SendEvent(ufp.TopicJobStatus, status)
}

func (a *fakeAgent) finishRelease(request ufp.ReleaseRequest) {
	status := ufp.JobStatus{JobID: request.JobID, Stage: ufp.StageRelease, Status: ufp.StatusSuccess}
	if a.failRun {
		status = ufp.JobStatus{JobID: request.JobID, Stage: ufp.StageRelease,
			Status: ufp.StatusFailed, Message: "port already bound"}
	}
	a.conn.SendEvent(ufp.TopicJobStatus, status)
}

type harness struct {
	store    *sqlite.Store
	pipeline *Pipeline
	agent    *fakeAgent
	vault    *secrets.Vault
}

func newHarness(t *testing.T) *harness {
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
		t.Fatalf("ca: %v", err)
	}
	if err := authority.EnsureLeaf(pki.Material{
		CertPath: cfg.ServerCertPath(), KeyPath: cfg.ServerKeyPath(),
		Names: []string{ufp.ServerName, "127.0.0.1"},
	}); err != nil {
		t.Fatalf("server certificate: %v", err)
	}
	caPEM, _ := authority.CertificatePEM()

	store, err := sqlite.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.CreateAgent(&models.Agent{ID: "a1", Name: "node-01", Key: "k",
		Roles: []models.Role{models.RoleBuilder, models.RoleRunner}}); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := store.SaveProject(&models.Project{
		Name: "api", Builder: "a1", Runners: []string{"a1"},
		Services: []models.Service{{GitURL: "git@host:api.git", Branch: "main", Dockerfile: "Dockerfile", Ports: []models.Port{{Host: 8080, Container: 80}}}},
	}); err != nil {
		t.Fatalf("save project: %v", err)
	}

	links := link.NewServer(cfg, store)
	links.SetRegistry(ufp.RegistryConfig{
		Host: "127.0.0.1:5000", Username: "uruflow", Password: "secret", CACert: caPEM,
	})
	if err := links.Start(); err != nil {
		t.Fatalf("start link: %v", err)
	}
	t.Cleanup(func() { links.Stop() })

	images := registry.New(registry.Options{
		Address: "127.0.0.1:5000", Namespace: "uruflow", CACert: caPEM,
	}, nil)

	vault, err := secrets.LoadOrCreateVault(filepath.Join(dir, "secrets.key"))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}

	releases := New(store, links, images, vault)
	links.Subscribe(releases)

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(caPEM))

	netConn, err := tls.DialWithDialer(&net.Dialer{Timeout: settleWindow}, "tcp", links.Addr(), &tls.Config{
		RootCAs: pool, ServerName: ufp.ServerName, MinVersion: tls.VersionTLS13,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	hello := ufp.Hello{AgentID: "a1", Hostname: "box", Version: "2.0.0",
		Roles: []ufp.Role{ufp.RoleBuilder, ufp.RoleRunner}}
	conn, _, err := ufp.Dial(netConn, hello, "k")
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	agent := &fakeAgent{
		conn: conn, builds: make(chan ufp.BuildRequest, 4), releases: make(chan ufp.ReleaseRequest, 4),
		operations: make(chan string, 4),
	}
	go conn.Serve(context.Background(), agent)
	if err := conn.SendEvent(ufp.TopicRegistryReady, ufp.Accepted{}); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "agent to come online", func() bool { return links.Online("a1") })
	return &harness{store: store, pipeline: releases, agent: agent, vault: vault}
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(settleWindow)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (h *harness) await(t *testing.T, releaseID string, status models.Status) *models.Release {
	t.Helper()
	var release *models.Release
	waitFor(t, "release "+releaseID+" to reach "+string(status), func() bool {
		loaded, err := h.store.GetRelease(releaseID)
		if err != nil {
			return false
		}
		release = loaded
		return loaded.Status == status
	})
	return release
}
