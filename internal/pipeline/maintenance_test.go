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
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/ufp"
	"strings"
	"testing"
	"time"
)

func TestRetentionKeepsDistinctServiceHistoryAndLivePins(t *testing.T) {
	releases := []models.Release{}
	reference := func(n int) string { return fmt.Sprintf("registry/uruflow/p-api@sha256:%064x", n) }
	for i := 0; i < 15; i++ {
		releases = append(releases, models.Release{ID: fmt.Sprint(i), Project: "p", Status: models.StatusSucceeded, Images: map[string]string{"api": reference(i)}})
	}
	releases = append([]models.Release{{ID: "duplicate", Project: "p", Status: models.StatusSucceeded, Images: map[string]string{"api": reference(0)}}}, releases...)
	projects := []models.Project{{Services: []models.Service{{Image: reference(14)}}}}
	containers := []models.Container{{Image: reference(13), Details: &ufp.ContainerDetails{Release: "12"}}}
	candidates, protected := retentionReferences(releases, projects, containers, 10, func(ref string) bool { return strings.HasPrefix(ref, "registry/") })
	old, keep := map[string]bool{}, map[string]bool{}
	for _, ref := range candidates {
		old[ref] = true
	}
	for _, ref := range protected {
		keep[ref] = true
	}
	if len(old) != 2 || !old[reference(10)] || !old[reference(11)] {
		t.Fatal("unexpected deletion candidates", old)
	}
	for _, i := range []int{0, 9, 12, 13, 14} {
		if !keep[reference(i)] {
			t.Fatal("lost protected image", i)
		}
	}
}
func TestMaintenanceGateExcludesReleasesAndReservations(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	pipeline := New(store, nil, nil, nil)
	ran, err := pipeline.registryMaintenance(func() error {
		if err := pipeline.claim(&models.Release{Project: "new"}, nil); !errors.Is(err, ErrRegistryMaintenance) {
			t.Fatalf("release entered maintenance: %v", err)
		}
		if err := pipeline.reserve("new"); !errors.Is(err, ErrRegistryMaintenance) {
			t.Fatalf("reservation entered maintenance: %v", err)
		}
		if _, err := pipeline.BeginRegistryUpdate(); !errors.Is(err, ErrRegistryMaintenance) {
			t.Fatalf("configuration entered maintenance: %v", err)
		}
		return nil
	})
	if !ran || err != nil {
		t.Fatal(ran, err)
	}
	finish, err := pipeline.BeginRegistryUpdate()
	if err != nil {
		t.Fatal(err)
	}
	ran, err = pipeline.registryMaintenance(func() error { t.Fatal("maintenance entered configuration update"); return nil })
	finish()
	if ran || err != nil {
		t.Fatal(ran, err)
	}
	if err = store.CreateRelease(&models.Release{ID: "active", Project: "p", Status: models.StatusBuilding, StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	ran, err = pipeline.registryMaintenance(func() error { t.Fatal("maintenance entered active release"); return nil })
	if ran || err != nil {
		t.Fatal(ran, err)
	}
	if err = pipeline.RegistryExclusive(func() error { t.Fatal("registry change entered active release"); return nil }); !errors.Is(err, ErrRegistryBusy) {
		t.Fatalf("registry change during release: %v", err)
	}
}
func TestExpiredArtifactsCannotBeRestored(t *testing.T) {
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for i := 0; i < 2; i++ {
		if err = store.CreateRelease(&models.Release{ID: fmt.Sprint(i), Project: "p", Image: "image", Status: models.StatusSucceeded, StartedAt: time.Now().Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := store.ReleaseRevision()
	if err = store.ExpireReleaseArtifacts([]string{"1"}); err != nil {
		t.Fatal(err)
	}
	after, _ := store.ReleaseRevision()
	if after <= before {
		t.Fatal("retention did not invalidate live release summaries")
	}
	release, err := store.GetRelease("1")
	if err != nil || !release.ArtifactsExpired {
		t.Fatal(release, err)
	}
	latest, err := store.LastSuccessfulRelease("p")
	if err != nil || latest.ID != "0" {
		t.Fatal(latest, err)
	}
	if _, err = New(store, nil, nil, nil).RollbackRelease("p", "1"); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatal("expired rollback allowed", err)
	}
}

func TestRecordedTagsMatchBothCommitAndManifest(t *testing.T) {
	commit := strings.Repeat("a", 40)
	reference := "registry/uruflow/app@sha256:" + strings.Repeat("b", 64)
	release := models.Release{Commit: strings.Repeat("c", 40), Commits: map[string]string{"api": commit}, Images: map[string]string{"api": reference, "prebuilt": "registry/uruflow/other@sha256:" + strings.Repeat("d", 64)}}
	tags := recordedTags([]models.Release{release})
	if len(tags) != 1 || !tags["registry/uruflow/app:"+commit[:12]+"@sha256:"+strings.Repeat("b", 64)] {
		t.Fatal("tag ownership did not preserve its recorded manifest", tags)
	}
}
