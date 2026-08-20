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

package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/projects"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()

	if os.Getenv("URUFLOW_DOCKER_TESTS") == "" {
		t.Skip("set URUFLOW_DOCKER_TESTS=1 to build a server")
	}

	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.UFPPort = 0
	cfg.Server.DataDir = dir
	cfg.Server.Advertise = "127.0.0.1"
	cfg.Registry.Host = "127.0.0.1"
	cfg.Registry.Port = 5602
	if err := cfg.Save(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	store, err := sqlite.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	server, err := NewServer(cfg, store)
	if err != nil {
		t.Skipf("server unavailable: %v", err)
	}
	store.CreateAgent(&models.Agent{ID: "b1", Name: "builder-01", Key: "k",
		Roles: []models.Role{models.RoleBuilder, models.RoleRunner}})
	return server, dir
}

func TestMissingKeyMaterialFailsClosed(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.DataDir = dir
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CreateAgent(&models.Agent{ID: "a1", Name: "runner-01", Key: "k",
		Roles: []models.Role{models.RoleRunner}}); err != nil {
		t.Fatal(err)
	}
	if err := validateKeyMaterial(cfg, store); err == nil {
		t.Fatal("missing CA was replaced while an agent was enrolled")
	}
	if err := store.DeleteAgent("a1"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSecret("token", []byte("encrypted")); err != nil {
		t.Fatal(err)
	}
	if err := validateKeyMaterial(cfg, store); err == nil || !strings.Contains(err.Error(), "secret encryption key") {
		t.Fatal("missing vault key was replaced while encrypted secrets existed")
	}
}

func TestDeletingAFileReportsAnOrphanedProject(t *testing.T) {
	server, _ := testServer(t)

	auto := true
	draft := projects.Draft{
		Project:    "api",
		Env:        "dev",
		Definition: projects.Definition{Git: "git@host:api.git"},
		Environment: projects.Environment{
			Branch: "main", Builder: "builder-01",
			Runners: []string{"builder-01"}, AutoDeploy: &auto,
		},
	}
	if err := server.Loader().Write(draft); err != nil {
		t.Fatalf("write: %v", err)
	}

	if loaded := server.ReloadProjects(); loaded != 1 {
		t.Fatalf("loaded %d projects", loaded)
	}
	if problems := server.ProjectProblems(); len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}

	_, environmentPath, _ := server.Loader().Paths("api", "dev")
	if err := os.Remove(environmentPath); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	server.ReloadProjects()

	problems := server.ProjectProblems()

	orphan := ""
	for _, problem := range problems {
		if strings.Contains(problem.Error(), "which is gone") {
			orphan = problem.Error()
		}
	}
	if orphan == "" {
		t.Fatalf("no orphan was reported: %v", problems)
	}
	if !strings.Contains(orphan, "api-dev") || !strings.Contains(orphan, "press d to remove") {
		t.Fatalf("orphan report is unhelpful: %s", orphan)
	}

	if _, err := server.Store().GetProject("api-dev"); err != nil {
		t.Error("the project was deleted implicitly when its file vanished")
	}
}
