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
)

func TestSecretsResolveIntoTheReleaseRequest(t *testing.T) {
	harness := newHarness(t)
	sealed, err := harness.vault.Seal("postgres://user:pass@db/api")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if err := harness.store.SetSecret("api_db", sealed); err != nil {
		t.Fatalf("store secret: %v", err)
	}
	project, _ := harness.store.GetProject("api")
	project.Runtime.Env = map[string]string{"DATABASE_URL": "${secret:api_db}", "MODE": "production"}
	harness.store.SaveProject(project)

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	<-harness.agent.builds
	select {
	case run := <-harness.agent.releases:
		env := run.Services[0].Env
		if env["DATABASE_URL"] != "postgres://user:pass@db/api" || env["MODE"] != "production" {
			t.Fatalf("release environment = %#v", env)
		}
	case <-time.After(settleWindow):
		t.Fatal("the runner never received the release")
	}
	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestMissingSecretFailsBeforeBuilding(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Runtime.Env = map[string]string{"DATABASE_URL": "${secret:absent}"}
	harness.store.SaveProject(project)

	_, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if !errors.Is(err, ErrSecretMissing) {
		t.Fatalf("error = %v, want ErrSecretMissing", err)
	}
	select {
	case build := <-harness.agent.builds:
		t.Fatalf("a build was dispatched despite the missing secret: %s", build.JobID)
	case <-time.After(500 * time.Millisecond):
	}
	releases, _ := harness.store.ListReleases(10)
	if len(releases) != 0 {
		t.Fatalf("a release was recorded for a rejected trigger: %+v", releases)
	}
}
