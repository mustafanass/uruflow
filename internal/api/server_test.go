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
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
)

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
