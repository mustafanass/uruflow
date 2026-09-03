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
	"strings"
	"sync"
	"time"

	"github.com/mustafanass/uruflow/internal/activity"
	"github.com/mustafanass/uruflow/internal/link"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/registry"
	"github.com/mustafanass/uruflow/internal/secrets"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/helper"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const (
	TagLatest = "latest"
)

var (
	ErrSecretMissing   = errors.New("secret is not set")
	ErrReleaseInFlight = errors.New("a release is already running for this project")
	ErrProjectBusy     = errors.New("another project operation is already running")
	ErrProjectNotFound = errors.New("project not found")
	ErrNoBuilder       = errors.New("project has no builder agent")
	ErrBuilderOffline  = errors.New("builder agent is offline")
	ErrNoRunners       = errors.New("project has no runner agents")
	ErrRunnerOffline   = errors.New("runner agent is offline")
	ErrNotRunner       = errors.New("agent does not have the runner role")
	ErrNothingToRun    = errors.New("no successful release to roll back to")
	ErrNotBuilder      = errors.New("agent does not have the builder role")
)

type Pipeline struct {
	store    storage.Store
	link     *link.Server
	registry *registry.Registry
	vault    *secrets.Vault
	activity *activity.Feed
	mu       sync.Mutex
	busy     map[string]struct{}
}

func New(store storage.Store, links *link.Server, images *registry.Registry, vault *secrets.Vault) *Pipeline {
	return &Pipeline{store: store, link: links, registry: images, vault: vault, busy: make(map[string]struct{})}
}

func (p *Pipeline) SetActivity(feed *activity.Feed) {
	p.activity = feed
}

func (p *Pipeline) secret(name string) (string, error) {
	sealed, err := p.store.GetSecret(name)
	if err != nil {
		return "", fmt.Errorf("%q: %w", name, ErrSecretMissing)
	}
	return p.vault.Open(sealed)
}

func (p *Pipeline) resolveEnv(env map[string]string) (map[string]string, error) {
	return secrets.Resolve(env, p.secret)
}

func (p *Pipeline) checkSecrets(project *models.Project) error {
	referenced := make(map[string]string)
	for key, value := range project.Runtime.Env {
		referenced[key] = value
	}
	for _, service := range project.ServiceList() {
		for key, value := range service.Env {
			referenced[key+"@"+service.Name] = value
		}
	}

	for _, name := range secrets.Names(referenced) {
		if _, err := p.store.GetSecret(name); err != nil {
			return fmt.Errorf("%s references %q: %w", project.Name, name, ErrSecretMissing)
		}
	}
	return nil
}

func (p *Pipeline) Trigger(projectName, commit string, trigger models.Trigger) (*models.Release, error) {
	if commit != "" && !models.ValidGitCommit(commit) {
		return nil, fmt.Errorf("invalid git commit %q", commit)
	}
	project, err := p.store.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
	}
	if err := p.ensureIdle(projectName); err != nil {
		return nil, err
	}

	if err := p.checkSecrets(project); err != nil {
		return nil, err
	}
	if err := p.validateProject(project); err != nil {
		return nil, err
	}
	if commit != "" {
		return nil, fmt.Errorf("project %s sources are selected per service; project-level commits are not supported", project.Name)
	}

	targets := p.buildTargets(project)
	workflow := project.EffectiveWorkflow()
	var runners []models.Agent
	if project.NeedsRunners() {
		runners, err = p.resolveRunners(project)
		if err != nil {
			return nil, err
		}
	}
	if workflow == models.WorkflowDeployOnly {
		services := project.ServiceList()
		prebuilt := make(map[string]string, len(services))
		for _, service := range services {
			prebuilt[service.Name] = service.Image
		}
		_, digest, _ := strings.Cut(services[0].Image, "@")
		release := &models.Release{
			ID:        helper.GenerateID(),
			Project:   project.Name,
			Image:     services[0].Image,
			Images:    prebuilt,
			Digest:    digest,
			Status:    models.StatusReleasing,
			Trigger:   trigger,
			Spec:      *project,
			StartedAt: time.Now(),
		}
		if err := p.claim(release, runners); err != nil {
			return nil, err
		}
		go p.rollout(release, &release.Spec, runners)
		return release, nil
	}

	builder, err := p.resolveBuilder(project)
	if err != nil {
		return nil, err
	}

	release := &models.Release{
		ID:          helper.GenerateID(),
		Project:     project.Name,
		Status:      models.StatusBuilding,
		Builder:     builder.ID,
		BuilderName: builder.Name,
		Trigger:     trigger,
		Spec:        *project,
		StartedAt:   time.Now(),
	}

	if err := p.claim(release, runners); err != nil {
		return nil, err
	}

	request := ufp.BuildRequest{
		JobID:   release.ID,
		Project: project.Name,
		Tags:    []string{TagLatest},
		Targets: targets,
	}

	logger.Info("[PIPELINE] release %s: building %s on %s", release.ID, project.Name, builder.Name)

	if err := p.dispatch(builder.ID, ufp.MethodBuildRun, request); err != nil {
		p.mu.Lock()
		p.failRelease(release, fmt.Sprintf("dispatch build to %s: %v", builder.Name, err))
		p.mu.Unlock()
		return nil, err
	}

	return release, nil
}

func (p *Pipeline) Rollback(projectName, imageRef string) (*models.Release, error) {
	project, err := p.store.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
	}
	if err := p.ensureIdle(projectName); err != nil {
		return nil, err
	}

	var source *models.Release
	if imageRef == "" {
		previous, err := p.store.LastSuccessfulRelease(projectName)
		if err != nil || previous.Image == "" {
			return nil, ErrNothingToRun
		}
		source = previous
		imageRef = previous.Image
	}
	spec := *project
	images := make(map[string]string)
	commits := make(map[string]string)
	digest := ""
	commit := ""
	if source != nil {
		if source.Spec.Name != "" {
			spec = source.Spec
		}
		images = source.Images
		if len(images) == 0 {
			images = map[string]string{"": source.Image}
		}
		digest = source.Digest
		commit = source.Commit
		commits = source.Commits
	} else {
		if !models.ValidDigestReference(imageRef) {
			return nil, fmt.Errorf("rollback image must use repository@sha256:digest")
		}
		builtServices := make([]models.Service, 0, 1)
		for _, service := range spec.ServiceList() {
			if service.Built() {
				builtServices = append(builtServices, service)
			}
		}
		if len(builtServices) != 1 {
			return nil, fmt.Errorf("an explicit image rollback requires exactly one built service")
		}
		images[builtServices[0].Name] = imageRef
		_, digest, _ = strings.Cut(imageRef, "@")
	}

	runners, err := p.resolveRunners(&spec)
	if err != nil {
		return nil, err
	}

	release := &models.Release{
		ID:        helper.GenerateID(),
		Project:   project.Name,
		Branch:    spec.Branch,
		Commit:    commit,
		Commits:   commits,
		Image:     imageRef,
		Images:    images,
		Digest:    digest,
		Status:    models.StatusReleasing,
		Trigger:   models.TriggerRollback,
		Spec:      spec,
		StartedAt: time.Now(),
	}

	if err := p.claim(release, runners); err != nil {
		return nil, err
	}

	logger.Info("[PIPELINE] release %s: rolling %s back to %s", release.ID, project.Name, imageRef)
	go p.rollout(release, &release.Spec, runners)

	return release, nil
}

func (p *Pipeline) Stop(projectName string) error {
	if err := p.reserve(projectName); err != nil {
		return err
	}
	defer p.releaseReservation(projectName)

	project, err := p.store.GetProject(projectName)
	if err != nil {
		return fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
	}

	runners, err := p.resolveRunners(project)
	if err != nil {
		return err
	}
	return p.dispatchProject(runners, ufp.MethodReleaseStop, project.Name)
}

func (p *Pipeline) Remove(projectName string) error {
	if err := p.reserve(projectName); err != nil {
		return err
	}
	defer p.releaseReservation(projectName)

	project, err := p.store.GetProject(projectName)
	if err != nil {
		return fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
	}

	runners, err := p.resolveRunners(project)
	if err != nil {
		return err
	}
	return p.dispatchProject(runners, ufp.MethodReleaseRemove, project.Name)
}

func (p *Pipeline) dispatchProject(runners []models.Agent, method, project string) error {
	failures := make([]error, len(runners))
	var group sync.WaitGroup
	for index := range runners {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			runner := runners[index]
			if err := p.dispatch(runner.ID, method, ufp.ProjectRef{Project: project}); err != nil {
				failures[index] = fmt.Errorf("%s: %w", runner.Name, err)
			}
		}(index)
	}
	group.Wait()
	return errors.Join(failures...)
}
