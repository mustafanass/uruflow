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

package services

import (
	"path/filepath"
	"testing"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
)

func TestAgentDeletionRequiresNoReferencesAndRevokesOnlineSession(t *testing.T) {
	store, err := sqlite.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CreateAgent(&models.Agent{ID: "a1", Name: "runner-01", Key: "k",
		Roles: []models.Role{models.RoleRunner}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProject(&models.Project{Name: "api", GitURL: "git@host:api.git",
		Builder: "a1", Runners: []string{"a1"}}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteAgent(store, nil, "a1"); err == nil {
		t.Fatal("referenced agent was deleted")
	}
	if err := store.DeleteProject("api"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAgentStatus("a1", models.AgentOnline); err != nil {
		t.Fatal(err)
	}
	if err := DeleteAgent(store, nil, "a1"); err == nil {
		t.Fatal("online agent was deleted without revocation")
	}
	revoked := false
	if err := DeleteAgent(store, func(string) { revoked = true }, "a1"); err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("online session was not revoked")
	}
	if _, err := store.GetAgent("a1"); err != storage.ErrNotFound {
		t.Fatalf("agent remained after deletion: %v", err)
	}
}
