/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 * This file is part of uruflow and is licensed under the MIT License.
 */

package pipeline

import (
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func TestMultiServiceRollbackReusesEveryDigest(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Services = []models.Service{
		{Name: "app", Dockerfile: "Dockerfile", Healthcheck: &models.Healthcheck{Type: "tcp", Port: 8080, Interval: time.Second, Timeout: time.Second, Retries: 3}, Labels: map[string]string{"monitor.team": "platform"}},
		{Name: "worker", Dockerfile: "Dockerfile.worker"},
		{Name: "cache", Image: prebuiltImage},
	}
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}

	first, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	<-harness.agent.builds
	initial := <-harness.agent.releases
	harness.await(t, first.ID, models.StatusSucceeded)

	rollback, err := harness.pipeline.Rollback("api", "")
	if err != nil {
		t.Fatal(err)
	}
	replayed := <-harness.agent.releases
	if len(replayed.Services) != len(initial.Services) {
		t.Fatalf("rollback services = %d, want %d", len(replayed.Services), len(initial.Services))
	}
	want := make(map[string]string, len(initial.Services))
	for _, service := range initial.Services {
		want[service.Name] = service.Image
	}
	for _, service := range replayed.Services {
		if service.Image != want[service.Name] {
			t.Fatalf("service %s image = %s, want %s", service.Name, service.Image, want[service.Name])
		}
		if service.Name == "app" && (service.Healthcheck == nil || service.Healthcheck.Type != "tcp" || service.Labels["monitor.team"] != "platform") {
			t.Fatalf("rollback lost app runtime specification: %+v", service)
		}
	}
	harness.await(t, rollback.ID, models.StatusSucceeded)
}
