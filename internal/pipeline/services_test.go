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
	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestMultiServiceBuildsEachAndReleasesTogether(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Services = []models.Service{
		{Name: "app", Dockerfile: "Dockerfile", Context: ".",
			Ports:       []models.Port{{Host: 8080, Container: 80}},
			Healthcheck: &models.Healthcheck{Type: "http", Scheme: "http", Path: "/ready", Port: 80, Interval: 2 * time.Second, Timeout: time.Second, Retries: 5},
			Labels:      map[string]string{"traefik.enable": "true"}},
		{Name: "worker", Dockerfile: "Dockerfile.worker", Command: "./worker"},
		{Name: "cache", Image: prebuiltImage, Volumes: []models.Volume{{Source: "/srv/redis", Target: "/data"}}},
	}
	project.Runtime.Env = map[string]string{"SHARED": "yes"}
	harness.store.SaveProject(project)

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	select {
	case build := <-harness.agent.builds:
		if len(build.Targets) != 2 {
			t.Fatalf("built %d targets, want 2: %+v", len(build.Targets), build.Targets)
		}
		byService := map[string]ufp.BuildTarget{}
		for _, target := range build.Targets {
			byService[target.Service] = target
		}
		if byService["app"].Image != "127.0.0.1:5000/uruflow/api-app" || byService["worker"].Dockerfile != "Dockerfile.worker" {
			t.Fatalf("build targets = %#v", byService)
		}
		if _, built := byService["cache"]; built {
			t.Error("a prebuilt service was sent to the builder")
		}
	case <-time.After(settleWindow):
		t.Fatal("no build was dispatched")
	}

	select {
	case run := <-harness.agent.releases:
		if len(run.Services) != 3 {
			t.Fatalf("released %d services, want 3: %+v", len(run.Services), run.Services)
		}
		byName := map[string]ufp.ServiceSpec{}
		for _, service := range run.Services {
			byName[service.Name] = service
		}
		if byName["cache"].Image != prebuiltImage || byName["app"].Image != "127.0.0.1:5000/uruflow/api-app@"+builtDigest {
			t.Fatalf("release images = %#v", byName)
		}
		if byName["worker"].Command != "./worker" || byName["app"].Env["SHARED"] != "yes" {
			t.Fatalf("runtime specification = %#v", byName)
		}
		if byName["app"].Healthcheck == nil || byName["app"].Healthcheck.Path != "/ready" || byName["app"].Healthcheck.Retries != 5 {
			t.Fatalf("healthcheck = %+v", byName["app"].Healthcheck)
		}
		if byName["app"].Labels["traefik.enable"] != "true" || byName["cache"].Restart != "unless-stopped" {
			t.Fatalf("labels/restart = %#v", byName)
		}
	case <-time.After(settleWindow):
		t.Fatal("no release was dispatched")
	}

	final := harness.await(t, release.ID, models.StatusSucceeded)
	if final.Spec.Services[0].Healthcheck == nil || final.Spec.Services[0].Labels["traefik.enable"] != "true" {
		t.Fatalf("release snapshot lost runtime specification: %+v", final.Spec.Services[0])
	}
}

func TestNativeBuildModelReachesBuilderAndRunner(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Networks = map[string]models.NetworkResource{"data": {Name: "api-data", Driver: "bridge", Internal: true}}
	project.Volumes = map[string]models.VolumeResource{"state": {Name: "api-state", Driver: "local"}}
	project.Services = []models.Service{
		{Name: "core", Dockerfile: "Dockerfile", DependsOn: []models.Dependency{{Service: "migrate", Condition: models.DependencyCompleted}}, Networks: []models.NetworkAttachment{{Name: "data", Aliases: []string{"core-api"}}}, Ports: []models.Port{{HostIP: "127.0.0.1", Host: 8080, Container: 8080}}, Resources: models.ResourceLimits{MemoryBytes: 256 << 20, CPUs: 1.5, PIDs: 128}, Security: models.Security{NoNewPrivileges: true, ReadOnlyRootFS: true, CapDrop: []string{"ALL"}}, Logging: models.LogConfig{Driver: "json-file", Options: map[string]string{"max-size": "10m"}}, Entrypoint: []string{"/app/core"}, CommandExec: []string{"serve"}},
		{Name: "migrate", GitURL: "git@host:database.git", Branch: "release", Dockerfile: "Dockerfile.migrate", Mode: models.ServiceModeJob, Job: models.Job{Timeout: 2 * time.Minute}, DependsOn: []models.Dependency{{Service: "database", Condition: models.DependencyHealthy}}, Networks: []models.NetworkAttachment{{Name: "data"}}},
		{Name: "database", Image: prebuiltImage, Networks: []models.NetworkAttachment{{Name: "data"}}, Volumes: []models.Volume{{Type: "volume", Source: "state", Target: "/data"}}, Healthcheck: &models.Healthcheck{Type: "command", Command: []string{"CMD", "check-ready"}, Interval: time.Second, Timeout: time.Second, Retries: 3, StartPeriod: 2 * time.Second}},
	}
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	build := <-harness.agent.builds
	if len(build.Targets) != 2 || build.Targets[1].GitURL != "git@host:database.git" || build.Targets[1].Branch != "release" {
		t.Fatalf("build targets = %#v", build.Targets)
	}
	run := <-harness.agent.releases
	if len(run.Services) != 3 || run.Services[0].Name != "database" || run.Services[1].Name != "migrate" || run.Services[2].Name != "core" {
		t.Fatalf("dependency order = %#v", run.Services)
	}
	core := run.Services[2]
	if run.Networks["data"].Name != "api-data" || run.Volumes["state"].Name != "api-state" || core.Ports[0].HostIP != "127.0.0.1" || core.Resources.MemoryBytes != 256<<20 || !core.Security.ReadOnlyRootFS || core.Logging.Options["max-size"] != "10m" || len(core.CommandExec) != 1 {
		t.Fatalf("native core spec = %#v; networks=%#v volumes=%#v", core, run.Networks, run.Volumes)
	}
	if run.Services[1].Mode != models.ServiceModeJob || run.Services[1].JobTimeout != 2*time.Minute || run.Services[0].Healthcheck == nil || run.Services[0].Healthcheck.StartPeriod != 2*time.Second {
		t.Fatalf("job/health model = %#v", run.Services)
	}
	harness.await(t, release.ID, models.StatusSucceeded)
}
