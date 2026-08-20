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
	"path/filepath"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestAgentRolesAndMetricsRoundTrip(t *testing.T) {
	store := newTestStore(t)

	agent := &models.Agent{ID: "a1", Name: "builder-01", Key: "key",
		Roles: []models.Role{models.RoleBuilder, models.RoleRunner}}
	if err := store.CreateAgent(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := store.SetAgentStatus("a1", models.AgentOnline); err != nil {
		t.Fatalf("set status: %v", err)
	}
	if err := store.SetAgentMetrics("a1", &models.Metrics{CPUPercent: 12.5}); err != nil {
		t.Fatalf("set metrics: %v", err)
	}

	loaded, err := store.GetAgent("a1")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if !loaded.HasRole(models.RoleBuilder) || !loaded.HasRole(models.RoleRunner) {
		t.Fatalf("roles = %v", loaded.Roles)
	}
	if loaded.Status != models.AgentOnline || loaded.Metrics.CPUPercent != 12.5 {
		t.Fatalf("status = %s cpu = %v", loaded.Status, loaded.Metrics.CPUPercent)
	}
}

func TestProjectRuntimeRoundTrip(t *testing.T) {
	store := newTestStore(t)

	project := &models.Project{
		Name: "api", GitURL: "git@host:api.git", Branch: "main",
		Builder: "a1", Runners: []string{"a1", "a2"}, AutoDeploy: true,
		Runtime: models.Runtime{
			Ports: []models.Port{{Host: 8080, Container: 80}},
			Env:   map[string]string{"MODE": "prod"},
		},
	}
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}

	project.Branch = "release"
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("resave project: %v", err)
	}

	loaded, err := store.GetProject("api")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if loaded.Branch != "release" || len(loaded.Runners) != 2 {
		t.Fatalf("branch = %s runners = %v", loaded.Branch, loaded.Runners)
	}
	if len(loaded.Runtime.Ports) != 1 || loaded.Runtime.Ports[0].Host != 8080 {
		t.Fatalf("ports = %+v", loaded.Runtime.Ports)
	}
	if loaded.Runtime.Env["MODE"] != "prod" {
		t.Fatalf("env = %v", loaded.Runtime.Env)
	}
}

func TestReleaseTargetsLogsAndRollbackSource(t *testing.T) {
	store := newTestStore(t)

	store.CreateAgent(&models.Agent{ID: "a1", Name: "runner-01", Key: "key",
		Roles: []models.Role{models.RoleRunner}})

	release := &models.Release{ID: "r1", Project: "api", Branch: "main",
		Status: models.StatusPending, Builder: "a1", BuilderName: "runner-01",
		Trigger: models.TriggerManual, StartedAt: time.Now()}
	if err := store.CreateRelease(release); err != nil {
		t.Fatalf("create release: %v", err)
	}

	store.SaveReleaseTarget(&models.ReleaseTarget{ReleaseID: "r1", AgentID: "a1",
		AgentName: "runner-01", Status: models.StatusPending})
	store.SaveReleaseTarget(&models.ReleaseTarget{ReleaseID: "r1", AgentID: "a1",
		AgentName: "runner-01", Status: models.StatusSucceeded})

	store.AppendLog(&models.LogLine{ReleaseID: "r1", Stage: "build",
		Stream: "stdout", Line: "step 1/3", Timestamp: time.Now()})

	ended := time.Now()
	release.Status = models.StatusSucceeded
	release.Image = "reg:5000/uruflow/api:abc1234"
	release.EndedAt = &ended
	if err := store.UpdateRelease(release); err != nil {
		t.Fatalf("update release: %v", err)
	}

	loaded, err := store.GetRelease("r1")
	if err != nil {
		t.Fatalf("get release: %v", err)
	}
	if len(loaded.Targets) != 1 || loaded.Targets[0].Status != models.StatusSucceeded {
		t.Fatalf("targets = %+v", loaded.Targets)
	}
	if loaded.EndedAt == nil {
		t.Fatal("ended_at was not persisted")
	}

	logs, _ := store.ListLogs("r1")
	if len(logs) != 1 || logs[0].Line != "step 1/3" {
		t.Fatalf("logs = %+v", logs)
	}

	previous, err := store.LastSuccessfulRelease("api")
	if err != nil || previous.Image != release.Image {
		t.Fatalf("rollback source = %+v err = %v", previous, err)
	}
}

func TestReplaceContainersAndStats(t *testing.T) {
	store := newTestStore(t)

	store.CreateAgent(&models.Agent{ID: "a1", Name: "runner-01", Key: "key",
		Roles: []models.Role{models.RoleRunner}})
	store.SetAgentStatus("a1", models.AgentOnline)
	store.SaveProject(&models.Project{Name: "api", GitURL: "git@host:api.git", Builder: "a1"})

	store.ReplaceContainers("a1", []models.Container{
		{ID: "c1", Name: "uruflow-api", Project: "api", State: "running"},
		{ID: "c2", Name: "uruflow-web", Project: "web", State: "exited"},
	})
	store.ReplaceContainers("a1", []models.Container{
		{ID: "c1", Name: "uruflow-api", Project: "api", State: "running"},
	})

	containers, _ := store.ListContainersByAgent("a1")
	if len(containers) != 1 || containers[0].Project != "api" {
		t.Fatalf("containers = %+v", containers)
	}

	succeeded := &models.Release{ID: "r1", Project: "api", Status: models.StatusSucceeded, StartedAt: time.Now()}
	failed := &models.Release{ID: "r2", Project: "api", Status: models.StatusFailed, StartedAt: time.Now()}
	store.CreateRelease(succeeded)
	store.CreateRelease(failed)

	store.CreateAlert(&models.Alert{ID: "al1", AgentID: "a1", AgentName: "runner-01",
		Type: "cpu", Message: "high cpu", Severity: models.SeverityWarning, CreatedAt: time.Now()})

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.AgentsOnline != 1 || stats.RunnersOnline != 1 || stats.BuildersOnline != 0 {
		t.Fatalf("agents online = %d runners = %d builders = %d",
			stats.AgentsOnline, stats.RunnersOnline, stats.BuildersOnline)
	}
	if stats.ProjectsTotal != 1 || stats.ReleasesTotal != 2 || stats.SuccessRate != 50 {
		t.Fatalf("projects = %d releases = %d rate = %v",
			stats.ProjectsTotal, stats.ReleasesTotal, stats.SuccessRate)
	}
	if stats.ContainersRunning != 1 || stats.AlertsActive != 1 {
		t.Fatalf("running = %d alerts = %d", stats.ContainersRunning, stats.AlertsActive)
	}

	store.ResolveAlert("al1")
	if active, _ := store.ListActiveAlerts(); len(active) != 0 {
		t.Fatalf("alert was not resolved: %+v", active)
	}
}

func TestDeleteAgentCascadesContainers(t *testing.T) {
	store := newTestStore(t)

	store.CreateAgent(&models.Agent{ID: "a1", Name: "runner-01", Key: "key",
		Roles: []models.Role{models.RoleRunner}})
	store.ReplaceContainers("a1", []models.Container{{ID: "c1", Name: "x", State: "running"}})

	if err := store.DeleteAgent("a1"); err != nil {
		t.Fatalf("delete agent: %v", err)
	}
	if containers, _ := store.ListContainers(); len(containers) != 0 {
		t.Fatalf("containers survived the agent: %+v", containers)
	}
	if err := store.DeleteAgent("a1"); err == nil {
		t.Fatal("deleting a missing agent should report not found")
	}
}
