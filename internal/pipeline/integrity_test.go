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

package pipeline

import (
	"errors"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestBuildStatusFromAnotherAgentIsRejected(t *testing.T) {
	harness := newHarness(t)
	harness.agent.holdBuild = make(chan struct{})
	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	build := <-harness.agent.builds
	harness.pipeline.JobStatus("another-agent", ufp.JobStatus{
		JobID: release.ID, Stage: ufp.StageBuild, Status: ufp.StatusSuccess,
		Image: builtImage, Images: map[string]string{"": builtImage}, Digest: builtDigest, Commit: builtCommit,
	})
	loaded, err := harness.store.GetRelease(release.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != models.StatusBuilding || loaded.Image != "" {
		t.Fatalf("forged event changed release: %+v", loaded)
	}
	close(harness.agent.holdBuild)
	_ = build
	<-harness.agent.releases
	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestMutableBuilderArtifactIsRejected(t *testing.T) {
	harness := newHarness(t)
	harness.agent.holdBuild = make(chan struct{})
	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	<-harness.agent.builds
	harness.pipeline.JobStatus("a1", ufp.JobStatus{
		JobID: release.ID, Stage: ufp.StageBuild, Status: ufp.StatusSuccess,
		Image:  "127.0.0.1:5000/uruflow/api:latest",
		Images: map[string]string{"": "127.0.0.1:5000/uruflow/api:latest"}, Commit: builtCommit,
	})
	final := harness.await(t, release.ID, models.StatusFailed)
	if final.Message == "" {
		t.Fatal("invalid artifact failure had no message")
	}
	close(harness.agent.holdBuild)
	select {
	case request := <-harness.agent.releases:
		t.Fatalf("mutable artifact reached runner: %+v", request)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestReleaseUsesTheClaimedProjectSnapshot(t *testing.T) {
	harness := newHarness(t)
	harness.agent.holdBuild = make(chan struct{})
	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	<-harness.agent.builds
	project, _ := harness.store.GetProject("api")
	project.Services[0].Ports[0].Host = 9090
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	close(harness.agent.holdBuild)
	select {
	case request := <-harness.agent.releases:
		if request.Services[0].Ports[0].Host != 8080 {
			t.Fatalf("release used edited port %d", request.Services[0].Ports[0].Host)
		}
	case <-time.After(settleWindow):
		t.Fatal("release was not dispatched")
	}
	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestOfflineConfiguredRunnerRejectsTheRelease(t *testing.T) {
	harness := newHarness(t)
	if err := harness.store.CreateAgent(&models.Agent{ID: "a2", Name: "node-02", Key: "k2",
		Roles: []models.Role{models.RoleRunner}}); err != nil {
		t.Fatal(err)
	}
	project, _ := harness.store.GetProject("api")
	project.Runners = append(project.Runners, "a2")
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if _, err := harness.pipeline.Trigger("api", "", models.TriggerManual); !errors.Is(err, ErrRunnerOffline) {
		t.Fatalf("error = %v, want ErrRunnerOffline", err)
	}
	releases, _ := harness.store.ListReleases(10)
	if len(releases) != 0 {
		t.Fatalf("rejected release was persisted: %+v", releases)
	}
}
