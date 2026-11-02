//go:build live

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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLiveRetentionCollectsOldBlobsAndPreservesSharedOCIImages(t *testing.T) {
	registry := liveRegistry(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := registry.Ensure(ctx, nil); err != nil {
		t.Fatal(err)
	}
	client, err := registry.httpClient()
	if err != nil {
		t.Fatal(err)
	}
	repository := "uruflow/retention"
	request := func(method, path, media string, body []byte, want int) *http.Response {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, "https://"+registry.options.Address+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.SetBasicAuth(registry.options.Username, registry.options.Password)
		req.Header.Set("Content-Type", media)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			message, _ := io.ReadAll(response.Body)
			response.Body.Close()
			t.Fatalf("%s %s: %s %s", method, path, response.Status, message)
		}
		return response
	}
	manifestSizes := map[string]int{}
	digest := func(body []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(body)) }
	push := func(tag, media string, body []byte) string {
		response := request(http.MethodPut, "/v2/"+repository+"/manifests/"+tag, media, body, 201)
		response.Body.Close()
		manifestSizes[digest(body)] = len(body)
		return digest(body)
	}
	image := func(name string) (string, string) {
		configuration := []byte(fmt.Sprintf(`{"architecture":"arm64","os":"linux","rootfs":{"type":"layers","diff_ids":[]},"config":{"Labels":{"fixture":%q}}}`, name))
		response := request(http.MethodPost, "/v2/"+repository+"/blobs/uploads/", "application/octet-stream", nil, 202)
		location, err := url.Parse(response.Header.Get("Location"))
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		query := location.Query()
		query.Set("digest", digest(configuration))
		response = request(http.MethodPut, location.EscapedPath()+"?"+query.Encode(), "application/octet-stream", configuration, 201)
		response.Body.Close()
		manifest, _ := json.Marshal(map[string]any{"schemaVersion": 2, "mediaType": ociManifestMediaType, "config": map[string]any{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": digest(configuration), "size": len(configuration)}, "layers": []any{}})
		return push(digest(manifest), ociManifestMediaType, manifest), digest(configuration)
	}
	old, oldBlob := image("old")
	shared, sharedBlob := image("shared")
	latest, latestBlob := image("latest")
	index := func(tag string, children ...string) string {
		descriptors := []any{}
		for _, child := range children {
			descriptors = append(descriptors, map[string]any{"mediaType": ociManifestMediaType, "digest": child, "size": manifestSizes[child]})
		}
		body, _ := json.Marshal(map[string]any{"schemaVersion": 2, "mediaType": ociIndexMediaType, "manifests": descriptors})
		return push(tag, ociIndexMediaType, body)
	}
	oldIndex := index(strings.Repeat("a", 12), old, shared)
	newIndex := index("latest", latest, shared)
	reference := func(digest string) string { return registry.options.Address + "/" + repository + "@" + digest }
	plan, err := registry.PlanPrune(ctx, []string{reference(oldIndex)}, []string{reference(newIndex)}, map[string]bool{registry.options.Address + "/" + repository + ":" + strings.Repeat("a", 12) + "@" + oldIndex: true})
	if err != nil || len(plan.Manifests) != 2 {
		t.Fatal(plan, err)
	}
	for _, manifest := range plan.Manifests {
		if err := registry.DeleteManifest(ctx, manifest); err != nil {
			t.Fatal(err)
		}
	}
	if err := registry.GarbageCollect(ctx); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{newIndex, shared, latest} {
		if _, err := registry.readManifest(ctx, repository, value); err != nil {
			t.Fatal("retained OCI manifest lost", value, err)
		}
	}
	response := request(http.MethodGet, "/v2/"+repository+"/blobs/"+oldBlob, "", nil, 404)
	response.Body.Close()
	for _, value := range []string{sharedBlob, latestBlob} {
		response := request(http.MethodGet, "/v2/"+repository+"/blobs/"+value, "", nil, 200)
		response.Body.Close()
	}
}
