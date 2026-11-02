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
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type ManifestRef struct {
	Repository string
	Digest     string
}
type PrunePlan struct {
	Roots     []string
	Manifests []ManifestRef
}

func (r *Registry) managedReference(reference string) (ManifestRef, bool) {
	prefix := r.options.Address + "/"
	if !strings.HasPrefix(reference, prefix) {
		return ManifestRef{}, false
	}
	repository, digest, ok := strings.Cut(strings.TrimPrefix(reference, prefix), "@")
	return ManifestRef{repository, digest}, ok && validRepository(repository) && digestName.MatchString(digest) && strings.HasPrefix(repository, r.options.Namespace+"/")
}
func (r *Registry) ManagedReference(reference string) bool {
	_, ok := r.managedReference(reference)
	return ok
}

func (r *Registry) PlanPrune(ctx context.Context, candidates, protected []string, automaticTags map[string]bool) (PrunePlan, error) {
	result := PrunePlan{}
	byRepository := map[string]map[string]bool{}
	keep := map[string][]string{}
	for _, reference := range candidates {
		if ref, ok := r.managedReference(reference); ok {
			if byRepository[ref.Repository] == nil {
				byRepository[ref.Repository] = map[string]bool{}
			}
			byRepository[ref.Repository][ref.Digest] = true
		}
	}
	for _, reference := range protected {
		if ref, ok := r.managedReference(reference); ok {
			keep[ref.Repository] = append(keep[ref.Repository], ref.Digest)
		}
	}
	repositories := make([]string, 0, len(byRepository))
	for repository := range byRepository {
		repositories = append(repositories, repository)
	}
	sort.Strings(repositories)
	for _, repository := range repositories {
		planned, err := r.planRepository(ctx, repository, byRepository[repository], keep[repository], automaticTags)
		if err != nil {
			return PrunePlan{}, err
		}
		result.Roots = append(result.Roots, planned.Roots...)
		result.Manifests = append(result.Manifests, planned.Manifests...)
	}
	return result, nil
}
func (r *Registry) planRepository(ctx context.Context, repository string, candidates map[string]bool, protected []string, automaticTags map[string]bool) (PrunePlan, error) {
	var result PrunePlan
	tags, err := r.Tags(ctx, repository)
	if err != nil {
		return result, err
	}
	records := map[string]manifestRecord{}
	failures := make([]error, len(tags))
	roots := make([]manifestRecord, len(tags))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for i := 0; i < catalogWorkers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				roots[index], failures[index] = r.readManifest(ctx, repository, tags[index])
			}
		}()
	}
	for index := range tags {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	for index, root := range roots {
		if failures[index] != nil {
			return result, failures[index]
		}
		records[root.Digest] = root
		if !candidates[root.Digest] || !automaticTags[r.options.Address+"/"+repository+":"+tags[index]+"@"+root.Digest] {
			protected = append(protected, root.Digest)
		}
	}
	load := func(digest string) (manifestRecord, error) {
		record, ok := records[digest]
		if ok {
			return record, nil
		}
		record, err := r.readManifest(ctx, repository, digest)
		if err == nil {
			records[digest] = record
		}
		return record, err
	}
	retained := map[string]bool{}
	var walk func(string, map[string]bool, int) error
	walk = func(digest string, seen map[string]bool, depth int) error {
		if seen[digest] {
			return nil
		}
		if depth > 8 {
			return fmt.Errorf("registry index nesting is too deep")
		}
		record, err := load(digest)
		if err != nil {
			return err
		}
		seen[digest] = true
		for _, child := range record.Children {
			if err := walk(child, seen, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, digest := range protected {
		if err := walk(digest, retained, 0); err != nil {
			return result, err
		}
	}
	deleted := map[string]bool{}
	for digest := range candidates {
		if retained[digest] {
			continue
		}
		closure := map[string]bool{}
		if err := walk(digest, closure, 0); err != nil {
			if errors.Is(err, ErrManifestNotFound) {
				continue
			}
			return result, err
		}
		result.Roots = append(result.Roots, r.options.Address+"/"+repository+"@"+digest)
		for child := range closure {
			if !retained[child] {
				deleted[child] = true
			}
		}
	}
	for digest := range deleted {
		result.Manifests = append(result.Manifests, ManifestRef{repository, digest})
	}
	sort.Slice(result.Manifests, func(i, j int) bool { return result.Manifests[i].Digest < result.Manifests[j].Digest })
	sort.Strings(result.Roots)
	return result, nil
}

func (r *Registry) DeleteManifest(ctx context.Context, reference ManifestRef) error {
	if !validRepository(reference.Repository) || !digestName.MatchString(reference.Digest) {
		return fmt.Errorf("invalid manifest deletion")
	}
	response, err := r.call(ctx, http.MethodDelete, "/v2/"+reference.Repository+"/manifests/"+reference.Digest)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusAccepted && response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete manifest: %s", response.Status)
	}
	return nil
}
