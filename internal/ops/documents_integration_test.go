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

package ops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
)

func TestDocumentsAuthorizeBeforeWritesAndDetectConflicts(t *testing.T) {
	if os.Getenv("URUFLOW_DOCKER_TESTS") == "" {
		t.Skip("set URUFLOW_DOCKER_TESTS=1 for Docker discovery")
	}
	cfg := config.Default()
	cfg.Server.DataDir = t.TempDir()
	cfg.Server.Advertise = "localhost"
	cfg.Registry.Host = "localhost"
	if err := cfg.Save(filepath.Join(cfg.Server.DataDir, "config.yaml")); err != nil {
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
	if err = store.CreateAgent(&models.Agent{ID: "runner", Name: "runner", Key: "fixture", Roles: []models.Role{models.RoleRunner}}); err != nil {
		t.Fatal(err)
	}
	engine := New(server)
	spec := "runners: [runner]\nservices:\n  api:\n    image: nginx@sha256:" + strings.Repeat("a", 64) + "\n"
	p, err := engine.CreateProject("api", "production", spec)
	if err != nil {
		t.Fatal(err)
	}
	original, err := engine.ProjectDocument(p.Name, false)
	if err != nil {
		t.Fatal(err)
	}
	denied := errors.New("policy denied")
	_, err = engine.SaveProjectDocument(p.Name, false, spec+"env:\n  MODE: changed\n", original.Revision, func(models.Project) error { return denied })
	if !errors.Is(err, denied) {
		t.Fatalf("policy not enforced: %v", err)
	}
	after, _ := engine.ProjectDocument(p.Name, false)
	loaded, _ := store.GetProject(p.Name)
	if after.Revision != original.Revision || loaded.Runtime.Env["MODE"] != "" {
		t.Fatal("authorization failure changed authoritative state")
	}
	saved, err := engine.SaveProjectDocument(p.Name, false, spec+"env:\n  MODE: production\n", original.Revision, nil)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision == original.Revision {
		t.Fatal("revision did not change")
	}
	if _, err = engine.SaveProjectDocument(p.Name, false, spec, original.Revision, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale write accepted: %v", err)
	}
	variables, err := engine.ProjectDocument(p.Name, true)
	if err != nil {
		t.Fatal(err)
	}
	saved, err = engine.SaveProjectDocument(p.Name, true, "secret TOKEN=fixture-sensitive-value\n", variables.Revision, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(saved.Content, "fixture-sensitive-value") || !strings.Contains(saved.Content, "${secret:api-production.TOKEN}") {
		t.Fatal("saved secret returned as plaintext")
	}
	cipher, err := store.GetSecret("api-production.TOKEN")
	if err != nil || strings.Contains(string(cipher), "fixture-sensitive-value") {
		t.Fatal("secret not encrypted at rest")
	}
	_, envPath := server.Loader().Paths("api", "production")
	info, err := os.Stat(envPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("variables file is not private")
	}
}
