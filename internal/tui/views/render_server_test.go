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

package views

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urustack/uruflow/internal/api"
	"github.com/urustack/uruflow/internal/config"
	"github.com/urustack/uruflow/internal/models"
	"github.com/urustack/uruflow/internal/storage/sqlite"
)

func liveServer(t *testing.T) *api.Server {
	t.Helper()

	if os.Getenv("URUFLOW_DOCKER_TESTS") == "" {
		t.Skip("set URUFLOW_DOCKER_TESTS=1 to render views that need a server")
	}

	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.UFPPort = 0
	cfg.Server.DataDir = dir
	cfg.Server.Advertise = "127.0.0.1"
	cfg.Registry.Host = "127.0.0.1"
	cfg.Registry.Port = 5599

	store, err := sqlite.New(filepath.Join(dir, "render.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	seedInto(store)

	server, err := api.NewServer(cfg, store)
	if err != nil {
		t.Skipf("server unavailable: %v", err)
	}
	return server
}

func seedInto(store *sqlite.Store) {
	store.CreateAgent(&models.Agent{ID: "a1", Name: "builder-01", Key: "k",
		Roles: []models.Role{models.RoleBuilder}})
	store.CreateAgent(&models.Agent{ID: "a2", Name: "web-01", Key: "k",
		Roles: []models.Role{models.RoleRunner}})
	store.SetAgentStatus("a1", models.AgentOnline)
	store.SetAgentMetrics("a1", &models.Metrics{CPUPercent: 34, MemoryPercent: 62,
		DiskPercent: 91, MemoryUsed: 8 << 30, MemoryTotal: 16 << 30,
		DiskUsed: 400 << 30, DiskTotal: 460 << 30, Uptime: 96000})

	store.SaveProject(&models.Project{Name: "api", GitURL: "git@github.com:acme/api.git",
		Branch: "main", Builder: "a1", Runners: []string{"a2"}, AutoDeploy: true,
		Runtime: models.Runtime{Ports: []models.Port{{Host: 8080, Container: 80}},
			Env: map[string]string{"MODE": "production"}}})
}

func TestRenderProjectsView(t *testing.T) {
	page := NewProjects(liveServer(t))
	page.Resize(renderWidth, renderHeight)
	page.Init()

	output := page.Render()
	checkFits(t, "projects", output)
	t.Log("\n" + output)
}

func TestRenderAgentsView(t *testing.T) {
	page := NewAgents(liveServer(t))
	page.Resize(renderWidth, renderHeight)
	page.Init()

	output := page.Render()
	checkFits(t, "agents", output)
	t.Log("\n" + output)
}

func TestRenderReleasesView(t *testing.T) {
	page := NewReleases(liveServer(t))
	page.Resize(renderWidth, renderHeight)
	page.Init()

	output := page.Render()
	checkFits(t, "releases", output)
	t.Log("\n" + output)
}

func TestRenderCreateTabs(t *testing.T) {
	page := NewProjects(liveServer(t))
	page.Resize(renderWidth, renderHeight)
	page.Init()
	page.openCreate()

	page.form.Field(fieldKind).Set("file")
	page.form.Field(fieldName).Set("api")
	page.form.Field(fieldRunners).SetValues([]string{"a2"})

	settings := page.Render()
	checkFits(t, "create settings", settings)
	t.Log("\n" + settings)

	page.tab = tabVariables
	page.variables.Load("LOG_LEVEL=debug\nDATABASE_URL=postgres://dev\n")
	env := page.Render()
	checkFits(t, "create env", env)
	t.Log("\n" + env)
}

func TestRenderDetailTabs(t *testing.T) {
	page := NewProjects(liveServer(t))
	page.Resize(renderWidth, renderHeight)
	page.Init()

	for name, tab := range map[string]int{"overview": detailOverview, "variables": detailVariables, "config": detailConfig} {
		page.detailTab = tab
		output := page.Render()
		checkFits(t, "detail "+name, output)
		t.Log("\n--- " + name + " ---\n" + output)
	}
}
