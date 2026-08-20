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
	"testing"
	"time"

	"github.com/urustack/uruflow/internal/models"
)

func TestReconcileClosesOutInterruptedWork(t *testing.T) {
	store := newTestStore(t)

	store.CreateAgent(&models.Agent{ID: "a1", Name: "node-01", Key: "k",
		Roles: []models.Role{models.RoleBuilder, models.RoleRunner}})
	store.SetAgentStatus("a1", models.AgentOnline)

	started := time.Now().Add(-90 * time.Second)
	store.CreateRelease(&models.Release{ID: "r1", Project: "api", Status: models.StatusBuilding,
		Builder: "a1", StartedAt: started})
	store.CreateRelease(&models.Release{ID: "r2", Project: "api", Status: models.StatusReleasing,
		Builder: "a1", StartedAt: started})
	finishedAt := started.Add(time.Second)
	store.CreateRelease(&models.Release{ID: "r3", Project: "api", Status: models.StatusSucceeded,
		Builder: "a1", StartedAt: started})
	store.UpdateRelease(&models.Release{ID: "r3", Status: models.StatusSucceeded,
		EndedAt: &finishedAt, Duration: 1234})

	store.SaveReleaseTarget(&models.ReleaseTarget{ReleaseID: "r2", AgentID: "a1",
		AgentName: "node-01", Status: models.StatusPending})
	store.SaveReleaseTarget(&models.ReleaseTarget{ReleaseID: "r3", AgentID: "a1",
		AgentName: "node-01", Status: models.StatusSucceeded})

	if err := store.SetAllAgentsOffline(); err != nil {
		t.Fatalf("reset agents: %v", err)
	}
	if err := store.FailUnfinishedReleases("interrupted"); err != nil {
		t.Fatalf("fail unfinished: %v", err)
	}

	agent, _ := store.GetAgent("a1")
	if agent.Status != models.AgentOffline {
		t.Fatalf("agent status = %s, want offline", agent.Status)
	}

	for _, id := range []string{"r1", "r2"} {
		release, err := store.GetRelease(id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		if release.Status != models.StatusFailed {
			t.Errorf("%s status = %s, want failed", id, release.Status)
		}
		if release.Message != "interrupted" || release.EndedAt == nil {
			t.Errorf("%s was not closed out: message=%q ended=%v", id, release.Message, release.EndedAt)
		}
		if release.Duration <= 0 {
			t.Errorf("%s duration = %d, want the elapsed time", id, release.Duration)
		}
	}

	interrupted, _ := store.GetRelease("r2")
	if interrupted.Targets[0].Status != models.StatusFailed {
		t.Errorf("pending target = %s, want failed", interrupted.Targets[0].Status)
	}

	finished, _ := store.GetRelease("r3")
	if finished.Status != models.StatusSucceeded || finished.Duration != 1234 {
		t.Errorf("a finished release was rewritten: %+v", finished)
	}
	if finished.Targets[0].Status != models.StatusSucceeded {
		t.Errorf("a finished target was rewritten: %+v", finished.Targets[0])
	}
}
