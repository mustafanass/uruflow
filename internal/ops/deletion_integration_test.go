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
	"strings"
	"testing"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
)

func TestDeletedProjectFilesGoFirstAndOtherEnvironmentsStay(t *testing.T) {
	engine, store := settingsEngine(t, legacyConfig(t))
	if err := store.CreateAgent(&models.Agent{ID: "runner", Name: "runner", Key: "fixture", Roles: []models.Role{models.RoleRunner}}); err != nil {
		t.Fatal(err)
	}
	spec := "runners: [runner]\nservices:\n  api:\n    image: nginx@sha256:" + strings.Repeat("a", 64) + "\n"
	for _, environment := range []string{"production", "staging"} {
		if _, err := engine.CreateProject("api", environment, spec); err != nil {
			t.Fatal(err)
		}
	}
	project, err := store.GetProject("api-production")
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.deleteProject(project); err != nil {
		t.Fatal(err)
	}
	removed, _ := engine.server.Loader().Paths("api", "production")
	kept, _ := engine.server.Loader().Paths("api", "staging")
	if _, err = os.Stat(removed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("environment file survived: %v", err)
	}
	if _, err = os.Stat(kept); err != nil {
		t.Fatalf("another environment lost its file: %v", err)
	}
	engine.ReloadProjects()
	if _, err = store.GetProject("api-production"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("reload restored the deleted project: %v", err)
	}
	if _, err = store.GetProject("api-staging"); err != nil {
		t.Fatalf("another environment was deleted: %v", err)
	}
	if problems := engine.server.ProjectProblems(); len(problems) != 0 {
		t.Fatalf("deletion left problems: %v", problems)
	}
}
