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
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

var ErrManifestNotFound = errors.New("registry manifest not found")
var repositoryPart = regexp.MustCompile(`^[a-z0-9]+(?:[._-]+[a-z0-9]+)*$`)
var tagName = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$`)
var digestName = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func validRepository(name string) bool {
	if name == "" || len(name) > 255 {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if !repositoryPart.MatchString(part) {
			return false
		}
	}
	return true
}
func validTag(tag string) bool { return tagName.MatchString(tag) }

type descriptor struct {
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}
type manifestRecord struct {
	Digest    string
	SizeKnown bool
	Children  []string
	Blobs     []descriptor
}

func (r *Registry) readManifest(ctx context.Context, repository, reference string) (manifestRecord, error) {
	var result manifestRecord
	if !validRepository(repository) || (!validTag(reference) && !digestName.MatchString(reference)) {
		return result, fmt.Errorf("invalid image reference")
	}
	response, err := r.call(ctx, http.MethodGet, "/v2/"+repository+"/manifests/"+reference)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return result, ErrManifestNotFound
	}
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("manifest %s: %s", repository, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil {
		return result, err
	}
	if len(body) > 8<<20 {
		return result, fmt.Errorf("registry manifest is too large")
	}
	var document struct {
		MediaType string       `json:"mediaType"`
		Config    descriptor   `json:"config"`
		Layers    []descriptor `json:"layers"`
		Manifests []descriptor `json:"manifests"`
		Subject   *descriptor  `json:"subject"`
	}
	if err = json.Unmarshal(body, &document); err != nil {
		return result, err
	}
	result.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	if digest := response.Header.Get("Docker-Content-Digest"); digest != "" && digest != result.Digest {
		return manifestRecord{}, fmt.Errorf("registry manifest digest mismatch")
	}
	if digestName.MatchString(reference) && reference != result.Digest {
		return manifestRecord{}, fmt.Errorf("requested manifest digest mismatch")
	}
	if document.MediaType == ociIndexMediaType || document.MediaType == dockerListMediaType || document.Manifests != nil {
		for _, child := range document.Manifests {
			if !digestName.MatchString(child.Digest) {
				return manifestRecord{}, fmt.Errorf("invalid child manifest digest")
			}
			result.Children = append(result.Children, child.Digest)
		}
	} else {
		if !digestName.MatchString(document.Config.Digest) || document.Layers == nil {
			return manifestRecord{}, fmt.Errorf("unsupported image manifest")
		}
		result.Blobs = append(result.Blobs, document.Config)
		result.Blobs = append(result.Blobs, document.Layers...)
		result.SizeKnown = true
		for _, blob := range result.Blobs {
			if !digestName.MatchString(blob.Digest) || blob.Size < 0 {
				return manifestRecord{}, fmt.Errorf("invalid image layer")
			}
		}
	}
	if document.Subject != nil {
		if !digestName.MatchString(document.Subject.Digest) {
			return manifestRecord{}, fmt.Errorf("invalid subject digest")
		}
		result.Children = append(result.Children, document.Subject.Digest)
	}
	return result, nil
}

func (r *Registry) Image(ctx context.Context, repository, tag string) (models.Image, error) {
	image := models.Image{Repository: repository, Tag: tag}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	root, err := r.readManifest(ctx, repository, tag)
	if err != nil {
		return image, err
	}
	image.Digest = root.Digest
	seen := map[string]bool{}
	blobs := map[string]int64{}
	var visit func(manifestRecord, int) bool
	visit = func(record manifestRecord, depth int) bool {
		if seen[record.Digest] {
			return true
		}
		if depth > 4 || len(seen) >= 128 {
			return false
		}
		seen[record.Digest] = true
		for _, blob := range record.Blobs {
			blobs[blob.Digest] = blob.Size
		}
		if len(record.Children) == 0 {
			return record.SizeKnown
		}
		complete := true
		for _, digest := range record.Children {
			child, err := r.readManifest(ctx, repository, digest)
			if err != nil {
				complete = false
				continue
			}
			if !visit(child, depth+1) {
				complete = false
			}
		}
		return complete
	}
	image.SizeKnown = visit(root, 0)
	if image.SizeKnown {
		for _, size := range blobs {
			image.Size += size
		}
	}
	r.catalogCache.mu.Lock()
	for i := range r.catalogCache.value.Images {
		if current := &r.catalogCache.value.Images[i]; current.Repository == repository && current.Tag == tag {
			*current = image
			break
		}
	}
	r.saveCatalogLocked()
	r.catalogCache.mu.Unlock()
	return image, nil
}
