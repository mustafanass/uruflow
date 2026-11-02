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

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/mustafanass/uruflow/internal/models"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func registryHTTP(t *testing.T, handler http.Handler) *Registry {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	return New(Options{Address: strings.TrimPrefix(server.URL, "https://"), Username: "user", Password: "pass", Namespace: "uruflow", CACert: string(certificate)}, nil)
}
func TestCatalogListsOnlyNamesAndTagsAndKeepsPartialResults(t *testing.T) {
	var manifests atomic.Int32
	registry := registryHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/_catalog":
			fmt.Fprint(w, `{"repositories":["fast","slow"]}`)
		case "/v2/fast/tags/list":
			fmt.Fprint(w, `{"tags":["first","latest"]}`)
		case "/v2/slow/tags/list":
			<-r.Context().Done()
		default:
			manifests.Add(1)
			http.Error(w, "unexpected manifest", 500)
		}
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	images, partial, err := registry.listImages(ctx)
	if err != nil || !partial || len(images) != 2 || manifests.Load() != 0 {
		t.Fatal(images, partial, err, manifests.Load())
	}
	for _, image := range images {
		if image.SizeKnown || image.Digest != "" {
			t.Fatal("list fetched manifest details", image)
		}
	}
}
func TestCatalogLimitsRepositoryConcurrency(t *testing.T) {
	var active, maximum atomic.Int32
	registry := registryHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/_catalog" {
			names := []string{}
			for i := 0; i < 24; i++ {
				names = append(names, fmt.Sprint("repo", i))
			}
			json.NewEncoder(w).Encode(map[string]any{"repositories": names})
			return
		}
		current := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); current > old && !maximum.CompareAndSwap(old, current); old = maximum.Load() {
		}
		time.Sleep(5 * time.Millisecond)
		fmt.Fprint(w, `{"tags":["latest"]}`)
	}))
	images, partial, err := registry.listImages(context.Background())
	if err != nil || partial || len(images) != 24 || maximum.Load() > catalogWorkers || maximum.Load() < 2 {
		t.Fatal(len(images), partial, err, maximum.Load())
	}
}
func TestOCIDetailsIncludeIndexLayersAndDeduplicateSharedBlobs(t *testing.T) {
	config := "sha256:" + strings.Repeat("a", 64)
	layer := "sha256:" + strings.Repeat("b", 64)
	child := fmt.Sprintf(`{"schemaVersion":2,"mediaType":%q,"config":{"digest":%q,"size":5},"layers":[{"digest":%q,"size":20}]}`, ociManifestMediaType, config, layer)
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(child)))
	index := fmt.Sprintf(`{"schemaVersion":2,"mediaType":%q,"manifests":[{"digest":%q,"size":100},{"digest":%q,"size":100}]}`, ociIndexMediaType, digest, digest)
	registry := registryHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, accept := range []string{ociIndexMediaType, ociManifestMediaType, dockerListMediaType, manifestMediaType} {
			if !strings.Contains(r.Header.Get("Accept"), accept) {
				t.Error("missing media type", accept)
			}
		}
		if strings.HasSuffix(r.URL.Path, "/latest") {
			fmt.Fprint(w, index)
		} else {
			fmt.Fprint(w, child)
		}
	}))
	image, err := registry.Image(context.Background(), "uruflow/app", "latest")
	if err != nil || !image.SizeKnown || image.Size != 25 || image.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(index))) {
		t.Fatal(image, err)
	}
}
func TestCatalogSnapshotIsImmediateAndPersistsAcrossRestart(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	registry := registryHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/_catalog" {
			close(started)
			<-release
			fmt.Fprint(w, `{"repositories":[]}`)
		}
	}))
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	registry.options.DataDir = filepath.Join(t.TempDir(), "registry")
	registry.catalogCache.value.Images = []models.Image{{Repository: "cached", Tag: "latest"}}
	registry.catalogCache.value.SampledAt = time.Now().Add(-time.Hour)
	registry.catalogCache.mu.Lock()
	registry.saveCatalogLocked()
	registry.catalogCache.mu.Unlock()
	loaded := New(registry.options, nil)
	value, _ := loaded.Images(context.Background())
	if len(value.Images) != 1 {
		t.Fatal("persisted catalog lost", value)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { registry.refreshCatalog(ctx); close(done) }()
	<-started
	snapshot, _ := registry.Images(context.Background())
	if len(snapshot.Images) != 1 || !snapshot.Refreshing {
		t.Fatal(snapshot)
	}
	close(release)
	<-done
	value, _ = registry.Images(context.Background())
	if len(value.Images) != 0 || value.Refreshing || value.Error != "" {
		t.Fatal(value)
	}
}

func TestRetentionPreservesSharedOCIChildrenAndManualTags(t *testing.T) {
	body := func(letter string) string {
		return fmt.Sprintf(`{"schemaVersion":2,"mediaType":%q,"config":{"digest":"sha256:%s","size":5},"layers":[]}`, ociManifestMediaType, strings.Repeat(letter, 64))
	}
	hash := func(body string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(body))) }
	oldChild, shared, newChild := body("a"), body("b"), body("c")
	index := func(first, second string) string {
		return fmt.Sprintf(`{"schemaVersion":2,"mediaType":%q,"manifests":[{"digest":%q,"size":1},{"digest":%q,"size":1}]}`, ociIndexMediaType, hash(first), hash(second))
	}
	oldIndex, newIndex := index(oldChild, shared), index(newChild, shared)
	documents := map[string]string{}
	for _, value := range []string{oldChild, shared, newChild, oldIndex, newIndex} {
		documents[hash(value)] = value
	}
	documents["1111111"] = oldIndex
	documents["2222222"] = newIndex
	documents["latest"] = newIndex
	documents["12345678"] = oldIndex
	var manual atomic.Bool
	registry := registryHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			tags := []string{"1111111", "2222222", "latest"}
			if manual.Load() {
				tags = append(tags, "12345678")
			}
			json.NewEncoder(w).Encode(map[string]any{"tags": tags})
			return
		}
		reference := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		if document, ok := documents[reference]; ok {
			fmt.Fprint(w, document)
		} else {
			http.NotFound(w, r)
		}
	}))
	reference := func(value string) string { return registry.options.Address + "/uruflow/app@" + hash(value) }
	plan, err := registry.PlanPrune(context.Background(), []string{reference(oldIndex)}, []string{reference(newIndex)}, map[string]bool{registry.options.Address + "/uruflow/app:1111111@" + hash(oldIndex): true})
	if err != nil || len(plan.Manifests) != 2 || len(plan.Roots) != 1 {
		t.Fatal(plan, err)
	}
	for _, manifest := range plan.Manifests {
		if manifest.Digest != hash(oldIndex) && manifest.Digest != hash(oldChild) {
			t.Fatal("shared or retained child selected for deletion", manifest)
		}
	}
	manual.Store(true)
	plan, err = registry.PlanPrune(context.Background(), []string{reference(oldIndex)}, []string{reference(newIndex)}, map[string]bool{registry.options.Address + "/uruflow/app:1111111@" + hash(oldIndex): true})
	if err != nil || len(plan.Manifests) != 0 {
		t.Fatal("manual tag was not retained", plan, err)
	}
}
