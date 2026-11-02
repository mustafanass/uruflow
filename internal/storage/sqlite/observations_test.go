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

package sqlite

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func TestSnapshotReleaseProjectionAndRevision(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	before, _ := store.ReleaseRevision()
	r := models.Release{ID: "r1", Project: "p1", Status: models.StatusSucceeded, StartedAt: time.Now(), Spec: models.Project{Runners: []string{"runner1"}, Services: []models.Service{{Name: "job", Mode: models.ServiceModeJob, Env: map[string]string{"SECRET": "never-on-the-wire"}}}}}
	if err = store.CreateRelease(&r); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveReleaseTarget(&models.ReleaseTarget{ReleaseID: r.ID, AgentID: "runner1", AgentName: "Runner 1", Status: models.StatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	after, _ := store.ReleaseRevision()
	if after <= before {
		t.Fatal("release revision did not change")
	}
	rows, err := store.SnapshotReleases()
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	if len(rows[0].Targets) != 1 || rows[0].RunnerCount != 1 || rows[0].Spec.Services[0].Env != nil {
		t.Fatal("projection lost outcomes or included configuration")
	}
	rows[0].Spec = models.Project{}
	wire, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	_ = json.Unmarshal(wire, &fields)
	if _, ok := fields["spec"]; ok {
		t.Fatal("snapshot included a release specification")
	}
}

func TestProjectHistoryKeepsRollbackCandidatesBeyondRecentFailures(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now()
	for i := 0; i < 105; i++ {
		release := models.Release{ID: fmt.Sprintf("r-%03d", i), Project: "project", Status: models.StatusFailed, StartedAt: now.Add(time.Duration(i) * time.Minute)}
		if i < 2 {
			release.Status = models.StatusSucceeded
			release.Images = map[string]string{"api": "image@sha256:artifact"}
		}
		if err := store.CreateRelease(&release); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := store.ProjectReleaseSummaries("project")
	if err != nil || len(rows) != 102 {
		t.Fatal("rollback candidates fell outside the history window", len(rows), err)
	}
	if rows[len(rows)-1].ID != "r-000" || rows[len(rows)-2].ID != "r-001" {
		t.Fatal("successful artifacts missing from history")
	}
	rows, err = store.ProjectReleaseSummaries("another")
	if err != nil || len(rows) != 0 {
		t.Fatal("project history included another project", rows, err)
	}
}
