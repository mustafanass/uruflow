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
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestImageExistenceUsesOnlyManifestHeaders(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, code := range []int{200, 404, 401, 403, 429, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			images := registryHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodHead || r.URL.Path != "/v2/uruflow/app/manifests/"+digest {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				user, password, ok := r.BasicAuth()
				if !ok || user != "user" || password != "pass" {
					t.Error("registry authentication missing")
				}
				for _, media := range []string{ociManifestMediaType, ociIndexMediaType, manifestMediaType, dockerListMediaType} {
					if !strings.Contains(r.Header.Get("Accept"), media) {
						t.Error("missing manifest media type", media)
					}
				}
				w.Header().Set("Docker-Content-Digest", digest)
				w.Header().Set("Content-Length", "8589934592")
				w.WriteHeader(code)
			}))
			exists, err := images.HasImage(context.Background(), images.Options().Address+"/uruflow/app@"+digest)
			if exists != (code == 200) || (err != nil) != (code != 200 && code != 404) || calls != 1 {
				t.Fatalf("exists=%v error=%v calls=%d", exists, err, calls)
			}
		})
	}
}

func TestImageExistenceRejectsWrongDigestAndRespectsDeadline(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	images := registryHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("b", 64))
	}))
	if exists, err := images.HasImage(context.Background(), images.Options().Address+"/uruflow/app@"+digest); exists || err == nil {
		t.Fatal("wrong manifest digest accepted")
	}
	if _, err := images.HasImage(context.Background(), "other.registry/app@"+digest); err == nil {
		t.Fatal("unmanaged reference accepted")
	}
	images = registryHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if exists, err := images.HasImage(ctx, images.Options().Address+"/uruflow/app@"+digest); exists || err == nil {
		t.Fatal("manifest deadline ignored")
	}
}
