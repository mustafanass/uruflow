//go:build integration

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

package web

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ops"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/internal/version"
	"golang.org/x/crypto/bcrypt"
)

func TestBrowserPreview(t *testing.T) {
	if os.Getenv("URUFLOW_BROWSER_QA") == "" {
		t.Skip("set URUFLOW_BROWSER_QA=1 for a local browser fixture")
	}
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.DataDir = dir
	cfg.Server.Advertise = "127.0.0.1"
	cfg.Registry.Host = "127.0.0.1"
	cfg.Registry.Port = 15999
	cfg.Web.Port = 19002
	if err := cfg.Save(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.New(cfg.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server, err := api.NewServer(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []models.Agent{{ID: "build-01", Name: "build-01", Roles: []models.Role{models.RoleBuilder}, Host: "10.40.0.11", Hostname: "build-eu-01", Platform: "linux/amd64", Version: version.Current}, {ID: "runner-01", Name: "runner-01", Roles: []models.Role{models.RoleRunner}, Host: "10.40.0.21", Hostname: "production-eu-01", Platform: "linux/amd64", Version: version.Current}, {ID: "runner-02", Name: "runner-02", Roles: []models.Role{models.RoleRunner}, Host: "10.40.0.22", Hostname: "production-eu-02", Platform: "linux/amd64", Version: version.Current}} {
		a.Key = "fixture-only"
		a.Status = models.AgentOnline
		if err = store.CreateAgent(&a); err != nil {
			t.Fatal(err)
		}
		if err = store.UpdateAgent(&a); err != nil {
			t.Fatal(err)
		}
	}
	if os.Getenv("URUFLOW_BROWSER_QA_LARGE") != "" {
		for index := 1; index <= 70; index++ {
			name := fmt.Sprintf("qa-runner-%02d", index)
			if err = store.CreateAgent(&models.Agent{ID: name, Name: name, Key: "fixture-only", Roles: []models.Role{models.RoleRunner}, Status: models.AgentOffline}); err != nil {
				t.Fatal(err)
			}
		}
	}
	engine := ops.New(server)
	spec := `builder: build-01
runners: [runner-01, runner-02]
services:
  api:
    git: https://github.com/example/commerce.git
    branch: main
    dockerfile: Dockerfile
    ports: ["8080:8080"]
    env:
      APP_ENV: production
  web:
    image: nginx@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    ports: ["8081:80"]
  worker:
    image: redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
`
	p, err := engine.CreateProject("commerce", "production", spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = engine.CreateProject("billing", "staging", "runners: [runner-02]\nservices:\n  api:\n    image: nginx@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"); err != nil {
		t.Fatal(err)
	}
	r := models.Release{ID: "release-20261101-a13bd85", Project: p.Name, Status: models.StatusSucceeded, Spec: *p, Commit: "a13bd85f23b05e98", Branch: "main", Builder: "build-01", BuilderName: "build-01", Trigger: models.TriggerManual, Image: "registry.example/commerce/api@sha256:aabbcc", Images: map[string]string{"api": "registry.example/commerce/api@sha256:aabbcc", "web": "nginx@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "worker": "redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}, StartedAt: time.Now().Add(-4 * time.Minute)}
	if err = store.CreateRelease(&r); err != nil {
		t.Fatal(err)
	}
	ended := time.Now().Add(-3 * time.Minute)
	r.EndedAt = &ended
	r.Duration = 60540
	if err = store.UpdateRelease(&r); err != nil {
		t.Fatal(err)
	}
	for _, u := range []storage.ConsoleUser{{Username: "admin", Role: "admin"}, {Username: "viewer", Role: "viewer", Projects: []string{p.Name}}, {Username: "operator", Role: "operator", Projects: []string{p.Name}}} {
		u.PasswordHash, _ = bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
		if err = store.CreateConsoleUser(&u); err != nil {
			t.Fatal(err)
		}
	}
	h, err := New(engine, store, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	seed := func() {
		now := time.Now()
		phase := float64(now.Unix() % 60)
		for i, id := range []string{"build-01", "runner-01", "runner-02"} {
			m := &models.Metrics{CollectedAt: now, MetricsSec: 2, Available: []string{"cpu", "memory", "disk", "load"}, ContainersAvailable: true, CPUCores: 8, HostUptime: 582145, CPUPercent: 12 + float64(i)*6 + math.Sin(phase/7)*8, MemoryUsed: uint64(3+i) * (1 << 30), MemoryTotal: 16 << 30, MemoryPercent: 24 + float64(i)*6, DiskTotal: 256 << 30, DiskUsed: 51 << 30, DiskPercent: 20, LoadAvg: []float64{1.2, 1.1, .8}}
			_ = store.SetAgentStatus(id, models.AgentOnline)
			_ = store.SetAgentMetrics(id, m)
			if i > 0 {
				var containers []models.Container
				for j, service := range []string{"api", "web", "worker"} {
					containers = append(containers, models.Container{ID: fmt.Sprintf("%02d%02d0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd", i, j), AgentID: id, Project: p.Name, Service: service, Name: "commerce-production-" + service, Image: r.Images[service], State: "running", Health: "healthy", ObservedAt: now, StatsAvailable: true, CPUPercent: 3 + float64(j) + math.Sin(phase/8), MemoryUsage: uint64(100+j*40) << 20, MemoryLimit: 512 << 20, NetworkRx: uint64(5000 + now.Unix()%10000), NetworkTx: 3450000, StartedAt: now.Add(-3 * time.Hour), PIDs: uint64(4 + j), Details: &ufp.ContainerDetails{Release: r.ID, Role: "runner", ImageID: "sha256:fixture", Ports: []string{"0.0.0.0:8080→8080/tcp"}, Networks: []ufp.ContainerNetwork{{Name: "commerce-production", Address: "172.18.0.4", Gateway: "172.18.0.1"}}, Mounts: []ufp.ContainerMount{{Type: "volume", Source: "commerce-data", Target: "/data"}}}})
				}
				_ = store.ReplaceContainers(id, containers)
			}
		}
	}
	seed()
	catalogCtx, catalogCancel := context.WithCancel(context.Background())
	defer catalogCancel()
	server.Registry().StartCatalog(catalogCtx)
	if err = h.Listen(cfg); err != nil {
		t.Fatal(err)
	}
	defer func() { h.Close() }()
	t.Log("Browser fixture: http://127.0.0.1:19002; test users admin, operator, viewer; test-only password: correct horse battery staple")

	stop := os.Getenv("URUFLOW_BROWSER_QA_STOP")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	duration := 30 * time.Minute
	if value := os.Getenv("URUFLOW_BROWSER_QA_DURATION"); value != "" {
		duration, err = time.ParseDuration(value)
		if err != nil || duration <= 0 {
			t.Fatal("URUFLOW_BROWSER_QA_DURATION must be a positive duration")
		}
	}
	deadline := time.After(duration)
	for {
		select {
		case <-ticker.C:
			seed()
			if restart := os.Getenv("URUFLOW_BROWSER_QA_RESTART"); restart != "" {
				if _, restartErr := os.Stat(restart); restartErr == nil {
					if err = os.Remove(restart); err != nil {
						t.Fatal(err)
					}
					h.Close()
					h, err = New(engine, store, dir, false)
					if err != nil {
						t.Fatal(err)
					}
					if err = h.Listen(cfg); err != nil {
						t.Fatal(err)
					}
					t.Log("Browser listener restarted with persisted sessions")
				}
			}
			if stop != "" {
				if _, err = os.Stat(stop); err == nil {
					return
				}
			}
		case <-deadline:
			return
		}
	}
}
