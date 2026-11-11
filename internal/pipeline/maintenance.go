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
	"fmt"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/pkg/logger"
)

func (p *Pipeline) StartRegistryMaintenance(ctx context.Context, retain func() int) {
	p.maintenanceDone = make(chan struct{})
	go func() {
		defer close(p.maintenanceDone)
		timer := time.NewTimer(10 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			window := retain()
			if window <= 0 {
				timer.Reset(15 * time.Minute)
				continue
			}
			pass, cancel := context.WithTimeout(ctx, 5*time.Minute)
			ran, err := p.maintainRegistry(pass, window)
			cancel()
			delay := 24 * time.Hour
			if err != nil {
				logger.Warn("[REGISTRY] maintenance: %v", err)
				delay = 15 * time.Minute
			} else if !ran {
				delay = 5 * time.Minute
			}
			timer.Reset(delay)
		}
	}()
}
func (p *Pipeline) WaitRegistryMaintenance(ctx context.Context) error {
	if p.maintenanceDone == nil {
		return nil
	}
	select {
	case <-p.maintenanceDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *Pipeline) registryMaintenance(run func() error) (bool, error) {
	p.mu.Lock()
	if p.maintenance || p.referenceEdits > 0 || len(p.busy) > 0 {
		p.mu.Unlock()
		return false, nil
	}
	active, err := p.store.ListActiveReleases()
	if err != nil {
		p.mu.Unlock()
		return false, err
	}
	if len(active) > 0 {
		p.mu.Unlock()
		return false, nil
	}
	p.maintenance = true
	p.mu.Unlock()
	defer func() { p.mu.Lock(); p.maintenance = false; p.mu.Unlock() }()
	return true, run()
}
func (p *Pipeline) maintainRegistry(ctx context.Context, retain int) (bool, error) {
	return p.registryMaintenance(func() error {
		releases, err := p.store.ArtifactHistory()
		if err != nil {
			return err
		}
		projects, err := p.store.ListProjects()
		if err != nil {
			return err
		}
		containers, err := p.store.ListContainers()
		if err != nil {
			return err
		}
		candidates, protected := retentionReferences(releases, projects, containers, retain, p.registry.ManagedReference)
		plan, err := p.registry.PlanPrune(ctx, candidates, protected, recordedTags(releases))
		if err != nil {
			return err
		}
		roots := map[string]bool{}
		for _, reference := range plan.Roots {
			roots[reference] = true
		}
		expired := []string{}
		for _, release := range releases {
			if release.ArtifactsExpired {
				continue
			}
			for _, reference := range releaseImages(release) {
				if roots[reference] {
					expired = append(expired, release.ID)
					break
				}
			}
		}
		if err = p.store.ExpireReleaseArtifacts(expired); err != nil {
			return fmt.Errorf("record registry retention: %w", err)
		}
		for _, manifest := range plan.Manifests {
			if err = p.registry.DeleteManifest(ctx, manifest); err != nil {
				return err
			}
		}
		logger.Info("[REGISTRY] maintenance: removed %d old manifests; collecting unreferenced layers", len(plan.Manifests))
		if err = p.registry.GarbageCollect(ctx); err != nil {
			return err
		}
		p.registry.RefreshCatalog()
		return nil
	})
}
func releaseImages(release models.Release) map[string]string {
	if len(release.Images) > 0 {
		return release.Images
	}
	if release.Image != "" {
		return map[string]string{"": release.Image}
	}
	return nil
}
func retentionReferences(releases []models.Release, projects []models.Project, containers []models.Container, retain int, managed func(string) bool) ([]string, []string) {
	protected := map[string]bool{}
	candidates := map[string]bool{}
	liveReleases := map[string]bool{}
	for _, container := range containers {
		if managed(container.Image) {
			protected[container.Image] = true
		}
		if container.Details != nil {
			liveReleases[container.Details.Release] = true
		}
	}
	for _, project := range projects {
		for _, service := range project.ServiceList() {
			if managed(service.Image) {
				protected[service.Image] = true
			}
		}
	}
	history := map[string]map[string]bool{}
	for _, release := range releases {
		for service, reference := range releaseImages(release) {
			if !managed(reference) {
				continue
			}
			key := release.Project + "\x00" + service
			if history[key] == nil {
				history[key] = map[string]bool{}
			}
			images := history[key]
			if liveReleases[release.ID] || !release.Status.Done() {
				protected[reference] = true
			}
			if release.Status == models.StatusSucceeded && !release.ArtifactsExpired && (images[reference] || len(images) < retain) {
				protected[reference] = true
				images[reference] = true
			} else {
				candidates[reference] = true
			}
		}
	}
	old, keep := []string{}, []string{}
	for reference := range candidates {
		if !protected[reference] {
			old = append(old, reference)
		}
	}
	for reference := range protected {
		keep = append(keep, reference)
	}
	return old, keep
}

func (p *Pipeline) RegistryExclusive(run func() error) error {
	ran, err := p.registryMaintenance(run)
	if err != nil {
		return err
	}
	if !ran {
		return ErrRegistryBusy
	}
	return nil
}

func (p *Pipeline) BeginRegistryUpdate() (func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.maintenance {
		return nil, ErrRegistryMaintenance
	}
	p.referenceEdits++
	return func() { p.mu.Lock(); p.referenceEdits--; p.mu.Unlock() }, nil
}

func recordedTags(releases []models.Release) map[string]bool {
	tags := map[string]bool{}
	for _, release := range releases {
		for service, reference := range releaseImages(release) {
			commit := release.Commits[service]
			if len(release.Commits) == 0 {
				commit = release.Commit
			}
			if !models.ValidGitCommit(commit) {
				continue
			}
			if len(commit) > 12 {
				commit = commit[:12]
			}
			repository, digest, ok := strings.Cut(reference, "@")
			if ok {
				tags[repository+":"+commit+"@"+digest] = true
			}
		}
	}
	return tags
}
