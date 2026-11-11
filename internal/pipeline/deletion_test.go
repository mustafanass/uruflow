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
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func TestDeleteWaitsForAnIdleFleetAndRemovesOnlyItsOwnResources(t *testing.T) {
	h := newHarness(t)
	project, err := h.store.GetProject("api")
	if err != nil {
		t.Fatal(err)
	}
	project.Networks = map[string]models.NetworkResource{
		"backend": {Name: "api_backend"},
		"shared":  {Name: "shared-net"},
		"edge":    {Name: "edge", External: true},
	}
	project.Volumes = map[string]models.VolumeResource{"data": {Name: "api_data"}}
	if err = h.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if err = h.store.SaveProject(&models.Project{Name: "web", Runners: []string{"a1"},
		Networks: map[string]models.NetworkResource{"shared": {Name: "shared-net"}}}); err != nil {
		t.Fatal(err)
	}
	cleaned := []string{}
	cleanup := func(deleted *models.Project) error {
		cleaned = append(cleaned, deleted.Name)
		return nil
	}

	if err = h.store.CreateRelease(&models.Release{ID: "web-release", Project: "web", Status: models.StatusBuilding, StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err = h.pipeline.Delete("api", false, false, cleanup); !errors.Is(err, ErrRegistryBusy) {
		t.Fatalf("delete during another release = %v", err)
	}
	if err = h.store.FailUnfinishedReleases("interrupted"); err != nil {
		t.Fatal(err)
	}
	if err = h.pipeline.Delete("api", true, false, cleanup); err == nil || !strings.Contains(err.Error(), "upgrade it") {
		t.Fatalf("volume removal on an older runner = %v", err)
	}
	select {
	case removal := <-h.agent.removals:
		t.Fatalf("a refused delete reached the runner: %+v", removal)
	default:
	}
	if len(cleaned) != 0 {
		t.Fatal("a refused delete ran its cleanup")
	}

	if err = h.pipeline.Delete("api", false, false, cleanup); err != nil {
		t.Fatal(err)
	}
	select {
	case removal := <-h.agent.removals:
		if removal.Project != "api" || strings.Join(removal.Networks, ",") != "api_backend" || len(removal.Volumes) != 0 {
			t.Fatalf("removal = %+v", removal)
		}
	case <-time.After(settleWindow):
		t.Fatal("the runner never received the removal")
	}
	if len(cleaned) != 1 || cleaned[0] != "api" {
		t.Fatalf("cleanup = %v", cleaned)
	}
}

func TestProjectImagesKeepWhatOtherProjectsStillUse(t *testing.T) {
	reference := func(n int) string { return fmt.Sprintf("registry/uruflow/api-web@sha256:%064x", n) }
	releases := []models.Release{
		{Project: "api", Images: map[string]string{"web": reference(1)}},
		{Project: "api", Images: map[string]string{"web": reference(2)}},
		{Project: "api", Images: map[string]string{"cache": "redis@sha256:" + strings.Repeat("b", 64)}},
		{Project: "billing", Images: map[string]string{"web": reference(2)}},
	}
	projects := []models.Project{{Name: "api", Services: []models.Service{{Image: reference(4)}}}}
	containers := []models.Container{{Project: "api", Image: reference(1)}, {Project: "billing", Image: reference(3)}}
	candidates, protected := projectImages("api", releases, projects, containers, func(ref string) bool { return strings.HasPrefix(ref, "registry/") })
	slices.Sort(candidates)
	if strings.Join(candidates, ",") != reference(1)+","+reference(4) {
		t.Fatalf("candidates = %v", candidates)
	}
	if !slices.Contains(protected, reference(2)) || !slices.Contains(protected, reference(3)) || slices.Contains(protected, reference(1)) {
		t.Fatalf("protected = %v", protected)
	}
}
