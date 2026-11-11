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
	"slices"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/internal/version"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const imageDeletionTimeout = 5 * time.Minute

func (p *Pipeline) Delete(projectName string, volumes, images bool, cleanup func(*models.Project) error) error {
	return p.RegistryExclusive(func() error {
		project, err := p.store.GetProject(projectName)
		if err != nil {
			return fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
		}
		runners, err := p.resolveRunners(project)
		if err != nil {
			return err
		}
		projects, err := p.store.ListProjects()
		if err != nil {
			return err
		}
		removal := projectRemoval(project, projects, volumes)
		if len(removal.Volumes) > 0 {
			for _, runner := range runners {
				if !version.AtLeast(runner.Version, ufp.ResourceRemovalSince) {
					return fmt.Errorf("%s runs %s; upgrade it to %s or later to delete volumes", runner.Name, runner.Version, ufp.ResourceRemovalSince)
				}
			}
		}
		if err := p.dispatchProject(runners, ufp.MethodReleaseRemove, removal); err != nil {
			return err
		}
		if images {
			if err := p.deleteImages(project.Name); err != nil {
				return err
			}
		}
		if err := cleanup(project); err != nil {
			return err
		}
		p.mu.Lock()
		delete(p.queued, project.Name)
		p.mu.Unlock()
		logger.Info("[PIPELINE] deleted %s", project.Name)
		return nil
	})
}

func (p *Pipeline) deleteImages(project string) error {
	ctx, cancel := context.WithTimeout(context.Background(), imageDeletionTimeout)
	defer cancel()
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
	candidates, protected := projectImages(project, releases, projects, containers, p.registry.ManagedReference)
	if len(candidates) == 0 {
		return nil
	}
	plan, err := p.registry.PlanPrune(ctx, candidates, protected, recordedTags(releases))
	if err != nil {
		return err
	}
	for _, manifest := range plan.Manifests {
		if err := p.registry.DeleteManifest(ctx, manifest); err != nil {
			return err
		}
	}
	logger.Info("[REGISTRY] deleted %d manifests of %s; collecting unreferenced layers", len(plan.Manifests), project)
	if err := p.registry.GarbageCollect(ctx); err != nil {
		return err
	}
	p.registry.RefreshCatalog()
	return nil
}

func projectRemoval(project *models.Project, projects []models.Project, volumes bool) ufp.ProjectRemoval {
	sharedNetworks, sharedVolumes := map[string]bool{}, map[string]bool{}
	for _, other := range projects {
		if other.Name == project.Name {
			continue
		}
		for _, network := range other.Networks {
			sharedNetworks[network.Name] = true
		}
		for _, volume := range other.Volumes {
			sharedVolumes[volume.Name] = true
		}
	}
	removal := ufp.ProjectRemoval{Project: project.Name}
	for _, network := range project.Networks {
		if !network.External && !sharedNetworks[network.Name] {
			removal.Networks = append(removal.Networks, network.Name)
		}
	}
	if volumes {
		for _, volume := range project.Volumes {
			if !volume.External && !sharedVolumes[volume.Name] {
				removal.Volumes = append(removal.Volumes, volume.Name)
			}
		}
	}
	slices.Sort(removal.Networks)
	slices.Sort(removal.Volumes)
	return removal
}

func projectImages(project string, releases []models.Release, projects []models.Project, containers []models.Container, managed func(string) bool) ([]string, []string) {
	candidates, protected := map[string]bool{}, map[string]bool{}
	for _, container := range containers {
		if container.Project != project && managed(container.Image) {
			protected[container.Image] = true
		}
	}
	for _, other := range projects {
		for _, service := range other.ServiceList() {
			if !managed(service.Image) {
				continue
			}
			if other.Name == project {
				candidates[service.Image] = true
			} else {
				protected[service.Image] = true
			}
		}
	}
	for _, release := range releases {
		for _, reference := range releaseImages(release) {
			if !managed(reference) {
				continue
			}
			if release.Project == project {
				candidates[reference] = true
			} else {
				protected[reference] = true
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
