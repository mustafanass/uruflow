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
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/registry"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func registryForReuse(t *testing.T, h *harness, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || !strings.Contains(r.URL.Path, "/manifests/") {
			t.Errorf("unexpected registry request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	h.pipeline.registry = registry.New(registry.Options{Address: strings.TrimPrefix(server.URL, "https://"), Namespace: "uruflow", CACert: string(certificate)}, nil)
}

func TestBuildReuseRequiresMatchingInputsAndSuccessfulArtifacts(t *testing.T) {
	h := newHarness(t)
	var missing atomic.Bool
	registryForReuse(t, h, func(w http.ResponseWriter, r *http.Request) {
		if missing.Load() && strings.Contains(r.URL.Path, "api-rust/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Docker-Content-Digest", builtDigest)
	})
	project, _ := h.store.GetProject("api")
	project.Services = []models.Service{
		{Name: "console", GitURL: "git@host:console.git", Branch: "main", Dockerfile: "Dockerfile", Watch: []string{"src", "package-lock.json"}, BuildArgs: map[string]string{"MODE": "production"}},
		{Name: "rust", GitURL: "git@host:rust.git", Branch: "main", Dockerfile: "Dockerfile"},
	}
	if err := h.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	previous := &models.Release{ID: "previous", Project: project.Name, Status: models.StatusSucceeded, Image: builtImage, StartedAt: time.Now(), Spec: *project, Images: map[string]string{}, Commits: map[string]string{}}
	for _, target := range h.pipeline.buildTargets(project) {
		previous.Images[target.Service] = target.Image + "@" + builtDigest
		previous.Commits[target.Service] = builtCommit
	}
	if err := h.store.CreateRelease(previous); err != nil {
		t.Fatal(err)
	}
	request := func(rebuild bool) ufp.BuildRequest {
		t.Helper()
		release, err := h.pipeline.Trigger(project.Name, "", models.TriggerManual, rebuild)
		if err != nil {
			t.Fatal(err)
		}
		var build ufp.BuildRequest
		select {
		case build = <-h.agent.builds:
		case <-time.After(settleWindow):
			t.Fatal("no build request")
		}
		final := h.await(t, release.ID, models.StatusSucceeded)
		<-h.agent.releases
		if len(final.Images) != 2 || len(final.Commits) != 2 {
			t.Fatal("release lost artifact coverage")
		}
		return build
	}
	first := request(false)
	for _, target := range first.Targets {
		if target.PreviousImage != previous.Images[target.Service] || target.PreviousCommit != builtCommit {
			t.Fatalf("no reuse candidate: %+v", target)
		}
	}
	project.Services[0].BuildArgs = map[string]string{"MODE": "staging"}
	if err := h.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	second := request(false)
	if second.Targets[0].PreviousImage != "" || second.Targets[1].PreviousImage == "" {
		t.Fatalf("build arguments invalidated the wrong services: %+v", second.Targets)
	}
	missing.Store(true)
	third := request(false)
	if third.Targets[0].PreviousImage == "" || third.Targets[1].PreviousImage != "" {
		t.Fatal("missing manifest did not rebuild only its service")
	}
	third = request(true)
	if !third.Rebuild {
		t.Fatal("forced rebuild flag was lost")
	}
	for _, target := range third.Targets {
		if target.PreviousImage != "" {
			t.Fatal("forced rebuild retained a candidate")
		}
	}
	restored, err := h.pipeline.RollbackRelease(project.Name, previous.ID)
	if err != nil {
		t.Fatal(err)
	}
	final := h.await(t, restored.ID, models.StatusSucceeded)
	if !maps.Equal(final.Images, previous.Images) || !maps.Equal(final.Commits, previous.Commits) || final.Spec.Services[0].BuildArgs["MODE"] != "production" {
		t.Fatal("rollback lost its complete artifact/configuration snapshot")
	}
}

func TestBuildReuseRejectsChangedBuildDefinitions(t *testing.T) {
	base := ufp.BuildTarget{Image: "registry/app", GitURL: "git@host:app.git", Branch: "main", Dockerfile: "Dockerfile", Context: ".", BuildArgs: map[string]string{"MODE": "production"}, Watch: []string{"src"}}
	changes := map[string]func(*ufp.BuildTarget){
		"repository": func(t *ufp.BuildTarget) { t.GitURL = "git@host:other.git" },
		"branch":     func(t *ufp.BuildTarget) { t.Branch = "staging" },
		"dockerfile": func(t *ufp.BuildTarget) { t.Dockerfile = "Dockerfile.other" },
		"context":    func(t *ufp.BuildTarget) { t.Context = "other" },
		"arguments":  func(t *ufp.BuildTarget) { t.BuildArgs = map[string]string{"MODE": "staging"} },
		"watch":      func(t *ufp.BuildTarget) { t.Watch = []string{"src", "shared"} },
		"registry":   func(t *ufp.BuildTarget) { t.Image = "other/app" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			next := base
			change(&next)
			if sameBuild(base, next) {
				t.Fatal("changed build definition was reusable")
			}
		})
	}
	h := newHarness(t)
	project, _ := h.store.GetProject("api")
	targets := h.pipeline.buildTargets(project)
	prior := &models.Release{ID: "expired", Project: project.Name, Status: models.StatusSucceeded, Image: builtImage, StartedAt: time.Now(), Spec: *project, Images: map[string]string{"": builtImage}, Commits: map[string]string{"": strings.Repeat("a", 40)}}
	if err := h.store.CreateRelease(prior); err != nil {
		t.Fatal(err)
	}
	if err := h.store.ExpireReleaseArtifacts([]string{prior.ID}); err != nil {
		t.Fatal(err)
	}
	if err := h.pipeline.reuseBuilds(project, targets); err != nil {
		t.Fatal(err)
	}
	if targets[0].PreviousImage != "" {
		t.Fatal("expired release was reused")
	}
}

func TestRegistryFailuresDoNotDispatchUnnecessaryBuilds(t *testing.T) {
	h := newHarness(t)
	registryForReuse(t, h, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	project, _ := h.store.GetProject("api")
	image := h.pipeline.buildTargets(project)[0].Image + "@" + builtDigest
	previous := &models.Release{ID: "previous", Project: project.Name, Spec: *project, Image: image, Images: map[string]string{"": image}, Commits: map[string]string{"": builtCommit}, Status: models.StatusSucceeded, StartedAt: time.Now()}
	if err := h.store.CreateRelease(previous); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pipeline.Trigger(project.Name, "", models.TriggerManual, false); !errors.Is(err, ErrRegistryUnavailable) {
		t.Fatalf("registry failure was not reported: %v", err)
	}
	select {
	case <-h.agent.builds:
		t.Fatal("registry failure dispatched a rebuild")
	default:
	}
	active, err := h.store.ListActiveReleases()
	if err != nil || len(active) != 0 {
		t.Fatalf("failed check left an active release: %+v %v", active, err)
	}
}

func TestManifestReuseChecksHaveBoundedConcurrency(t *testing.T) {
	h := newHarness(t)
	var active, maximum atomic.Int32
	registryForReuse(t, h, func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); current > old && !maximum.CompareAndSwap(old, current); old = maximum.Load() {
		}
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Docker-Content-Digest", builtDigest)
	})
	targets := make([]ufp.BuildTarget, 14)
	for i := range targets {
		image := h.pipeline.registry.ImageRepository(fmt.Sprint("app-", i))
		targets[i] = ufp.BuildTarget{Service: fmt.Sprint("app-", i), Image: image, PreviousImage: image + "@" + builtDigest, PreviousCommit: builtCommit}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.pipeline.checkReusableImages(ctx, targets); err != nil {
		t.Fatal(err)
	}
	if maximum.Load() > 8 || maximum.Load() < 2 {
		t.Fatalf("manifest concurrency: %d", maximum.Load())
	}
}
