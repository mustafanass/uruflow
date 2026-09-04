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
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func TestBuildThenReleaseReachesRunners(t *testing.T) {
	harness := newHarness(t)
	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if release.Status != models.StatusBuilding {
		t.Fatalf("release starts as %s, want building", release.Status)
	}

	select {
	case build := <-harness.agent.builds:
		if len(build.Targets) != 1 || build.Targets[0].Image != "127.0.0.1:5000/uruflow/api" {
			t.Fatalf("build targets = %+v", build.Targets)
		}
		if len(build.Tags) != 1 || build.Tags[0] != TagLatest {
			t.Fatalf("extra tags = %v", build.Tags)
		}
		if build.Targets[0].Dockerfile != "Dockerfile" || build.Targets[0].Context != "." {
			t.Fatalf("build defaults = %q %q", build.Targets[0].Dockerfile, build.Targets[0].Context)
		}
		if build.Timeout != 90*time.Minute {
			t.Fatalf("build timeout = %s", build.Timeout)
		}
	case <-time.After(settleWindow):
		t.Fatal("the builder never received build.run")
	}

	select {
	case run := <-harness.agent.releases:
		if len(run.Services) != 1 || run.Services[0].Image != builtImage {
			t.Fatalf("runner received %+v, want the freshly built tag", run.Services)
		}
		if len(run.Services[0].Ports) != 1 || run.Services[0].Ports[0].Host != 8080 {
			t.Fatalf("runtime ports = %+v", run.Services[0].Ports)
		}
		if run.Timeout <= 0 || run.Timeout > 90*time.Minute {
			t.Fatalf("remaining release timeout = %s", run.Timeout)
		}
	case <-time.After(settleWindow):
		t.Fatal("the runner never received release.run")
	}

	final := harness.await(t, release.ID, models.StatusSucceeded)
	if final.Image != builtImage || final.Commit != builtCommit {
		t.Fatalf("release recorded image %q commit %q", final.Image, final.Commit)
	}
	if len(final.Targets) != 1 || final.Targets[0].Status != models.StatusSucceeded {
		t.Fatalf("targets = %+v", final.Targets)
	}
	if final.Duration <= 0 || final.EndedAt == nil {
		t.Fatalf("release was not closed out: duration=%d ended=%v", final.Duration, final.EndedAt)
	}
	logs, _ := harness.store.ListLogs(release.ID, 0, 100)
	if len(logs) == 0 {
		t.Fatal("build logs were not persisted")
	}
}

func TestBuildOnlyStopsAfterPublishingArtifacts(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Workflow = models.WorkflowBuildOnly
	project.Runners = nil
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-harness.agent.builds:
	case <-time.After(settleWindow):
		t.Fatal("build-only workflow never reached the builder")
	}
	final := harness.await(t, release.ID, models.StatusSucceeded)
	if len(final.Targets) != 0 || final.Image != builtImage {
		t.Fatalf("build-only result = targets:%+v image:%q", final.Targets, final.Image)
	}
	select {
	case run := <-harness.agent.releases:
		t.Fatalf("build-only workflow dispatched release.run: %+v", run)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestReleaseOnlySkipsBuilder(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Workflow = models.WorkflowDeployOnly
	project.Builder = ""
	project.Services = []models.Service{{Name: "cache", Image: prebuiltImage}}
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if release.Status != models.StatusReleasing {
		t.Fatalf("release-only workflow started as %q", release.Status)
	}
	select {
	case build := <-harness.agent.builds:
		t.Fatalf("release-only workflow dispatched build.run: %+v", build)
	case run := <-harness.agent.releases:
		if len(run.Services) != 1 || run.Services[0].Image != prebuiltImage {
			t.Fatalf("release-only services = %+v", run.Services)
		}
	case <-time.After(settleWindow):
		t.Fatal("release-only workflow never reached the runner")
	}
	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestFailedBuildNeverReachesRunners(t *testing.T) {
	harness := newHarness(t)
	harness.agent.failBuild = true
	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	final := harness.await(t, release.ID, models.StatusFailed)
	if final.Message != "compile error" {
		t.Fatalf("failure message = %q", final.Message)
	}
	select {
	case run := <-harness.agent.releases:
		t.Fatalf("a failed build still rolled out %+v", run.Services)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestFailedRunnerFailsTheRelease(t *testing.T) {
	harness := newHarness(t)
	harness.agent.failRun = true
	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	final := harness.await(t, release.ID, models.StatusFailed)
	if len(final.Targets) != 1 || final.Targets[0].Status != models.StatusFailed || final.Targets[0].Message != "port already bound" {
		t.Fatalf("targets = %+v", final.Targets)
	}
}

func TestRollbackSkipsTheBuildStage(t *testing.T) {
	harness := newHarness(t)
	first, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	<-harness.agent.builds
	<-harness.agent.releases
	harness.await(t, first.ID, models.StatusSucceeded)

	rollback, err := harness.pipeline.Rollback("api", "")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rollback.Image != builtImage {
		t.Fatalf("rollback image = %q", rollback.Image)
	}
	select {
	case build := <-harness.agent.builds:
		t.Fatalf("rollback rebuilt %q instead of reusing the image", build.Project)
	case run := <-harness.agent.releases:
		if run.Services[0].Image != builtImage {
			t.Fatalf("rollback released %q", run.Services[0].Image)
		}
	case <-time.After(settleWindow):
		t.Fatal("rollback never reached a runner")
	}
	harness.await(t, rollback.ID, models.StatusSucceeded)
}
