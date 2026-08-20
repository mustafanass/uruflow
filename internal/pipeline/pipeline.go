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
	"errors"
	"fmt"
	"sync"
	"time"

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
	dispatchTimeout     = 30 * time.Second
	TagLatest           = "latest"
	activeReleaseWindow = 50
)

var (
	ErrSecretMissing   = errors.New("secret is not set")
	ErrReleaseInFlight = errors.New("a release is already running for this project")
	ErrProjectNotFound = errors.New("project not found")
	ErrNoBuilder       = errors.New("project has no builder agent")
	ErrBuilderOffline  = errors.New("builder agent is offline")
	ErrNoRunners       = errors.New("project has no runner agents")
	ErrNothingToRun    = errors.New("no successful release to roll back to")
	ErrNotBuilder      = errors.New("agent does not have the builder role")
)

type Pipeline struct {
	store    storage.Store
	link     *link.Server
	registry *registry.Registry
	vault    *secrets.Vault
	mu       sync.Mutex
}

func New(store storage.Store, links *link.Server, images *registry.Registry, vault *secrets.Vault) *Pipeline {
	return &Pipeline{store: store, link: links, registry: images, vault: vault}
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

	builder, err := p.resolveBuilder(project)
	if err != nil {
		return nil, err
	}

	runners, err := p.resolveRunners(project)
	if err != nil {
		return nil, err
	}

	release := &models.Release{
		ID:          helper.GenerateID(),
		Project:     project.Name,
		Branch:      project.Branch,
		Commit:      commit,
		Status:      models.StatusBuilding,
		Builder:     builder.ID,
		BuilderName: builder.Name,
		Trigger:     trigger,
		StartedAt:   time.Now(),
	}

	if err := p.claim(release, runners); err != nil {
		return nil, err
	}

	request := ufp.BuildRequest{
		JobID:   release.ID,
		Project: project.Name,
		GitURL:  project.GitURL,
		Branch:  project.Branch,
		Commit:  commit,
		Tags:    []string{TagLatest},
		Targets: p.buildTargets(project),
	}

	logger.Info("[PIPELINE] release %s: building %s on %s", release.ID, project.Name, builder.Name)

	if err := p.dispatch(builder.ID, ufp.MethodBuildRun, request); err != nil {
		p.failRelease(release, fmt.Sprintf("dispatch build to %s: %v", builder.Name, err))
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

	if imageRef == "" {
		previous, err := p.store.LastSuccessfulRelease(projectName)
		if err != nil || previous.Image == "" {
			return nil, ErrNothingToRun
		}
		imageRef = previous.Image
	}

	runners, err := p.resolveRunners(project)
	if err != nil {
		return nil, err
	}

	release := &models.Release{
		ID:        helper.GenerateID(),
		Project:   project.Name,
		Branch:    project.Branch,
		Image:     imageRef,
		Status:    models.StatusReleasing,
		Trigger:   models.TriggerRollback,
		StartedAt: time.Now(),
	}

	if err := p.claim(release, runners); err != nil {
		return nil, err
	}

	logger.Info("[PIPELINE] release %s: rolling %s back to %s", release.ID, project.Name, imageRef)
	go p.rollout(release, project, runners)

	return release, nil
}

func (p *Pipeline) Stop(projectName string) error {
	project, err := p.store.GetProject(projectName)
	if err != nil {
		return fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
	}

	var failure error
	for _, agentID := range project.Runners {
		if !p.link.Online(agentID) {
			continue
		}
		if err := p.dispatch(agentID, ufp.MethodReleaseStop, ufp.ProjectRef{Project: project.Name}); err != nil {
			failure = err
		}
	}
	return failure
}

func (p *Pipeline) Remove(projectName string) error {
	project, err := p.store.GetProject(projectName)
	if err != nil {
		return fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
	}

	var failure error
	for _, agentID := range project.Runners {
		if !p.link.Online(agentID) {
			continue
		}
		if err := p.dispatch(agentID, ufp.MethodReleaseRemove, ufp.ProjectRef{Project: project.Name}); err != nil {
			failure = err
		}
	}
	return failure
}

func (p *Pipeline) rollout(release *models.Release, project *models.Project, runners []models.Agent) {
	request, err := p.releaseRequest(release, project)
	if err != nil {
		p.failRelease(release, err.Error())
		return
	}

	for _, runner := range runners {
		if !p.link.Online(runner.ID) {
			p.finishTarget(release.ID, runner.ID, runner.Name, models.StatusSkipped, "agent offline")
			continue
		}
		if err := p.dispatch(runner.ID, ufp.MethodReleaseRun, *request); err != nil {
			p.finishTarget(release.ID, runner.ID, runner.Name, models.StatusFailed, err.Error())
		}
	}

	p.settle(release.ID)
}

func (p *Pipeline) buildTargets(project *models.Project) []ufp.BuildTarget {
	targets := make([]ufp.BuildTarget, 0, len(project.ServiceList()))

	for _, service := range project.ServiceList() {
		if !service.Built() {
			continue
		}
		targets = append(targets, ufp.BuildTarget{
			Service:    service.Name,
			Image:      p.imageRepository(project, service),
			Dockerfile: service.BuildFile(),
			Context:    service.BuildContext(),
			BuildArgs:  service.BuildArgs,
		})
	}

	return targets
}

func (p *Pipeline) imageRepository(project *models.Project, service models.Service) string {
	if service.Name == "" {
		return p.registry.ImageRepository(project.Name)
	}
	return p.registry.ImageRepository(project.Name + "-" + service.Name)
}

func (p *Pipeline) releaseRequest(release *models.Release, project *models.Project) (*ufp.ReleaseRequest, error) {
	built := release.Images
	if len(built) == 0 {
		built = map[string]string{"": release.Image}
	}

	request := &ufp.ReleaseRequest{JobID: release.ID, Project: project.Name}

	for _, service := range project.ServiceList() {
		image := service.Image
		if service.Built() {
			resolved, ok := built[service.Name]
			if !ok {
				return nil, fmt.Errorf("no image was built for service %q", service.Name)
			}
			image = resolved
		}

		env, err := p.resolveEnv(project.ServiceEnv(service))
		if err != nil {
			return nil, err
		}

		spec := ufp.ServiceSpec{
			Name:    service.Name,
			Image:   image,
			Env:     env,
			Network: service.Network,
			Restart: service.RestartPolicy(),
			Command: service.Command,
		}
		for _, port := range service.Ports {
			spec.Ports = append(spec.Ports, ufp.PortBinding{
				Host: port.Host, Container: port.Container, Protocol: port.Protocol,
			})
		}
		for _, volume := range service.Volumes {
			spec.Volumes = append(spec.Volumes, ufp.VolumeBinding{
				Source: volume.Source, Target: volume.Target, ReadOnly: volume.ReadOnly,
			})
		}

		request.Services = append(request.Services, spec)
	}

	return request, nil
}

func (p *Pipeline) dispatch(agentID, method string, payload any) error {
	ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
	defer cancel()

	_, err := p.link.Request(ctx, agentID, method, payload)
	return err
}

func (p *Pipeline) resolveBuilder(project *models.Project) (*models.Agent, error) {
	if project.Builder == "" {
		return nil, fmt.Errorf("%s: %w", project.Name, ErrNoBuilder)
	}

	builder, err := p.store.GetAgent(project.Builder)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", project.Name, ErrNoBuilder)
	}
	if !builder.HasRole(models.RoleBuilder) {
		return nil, fmt.Errorf("%s: %w", builder.Name, ErrNotBuilder)
	}
	if !p.link.Online(builder.ID) {
		return nil, fmt.Errorf("%s: %w", builder.Name, ErrBuilderOffline)
	}
	return builder, nil
}

func (p *Pipeline) resolveRunners(project *models.Project) ([]models.Agent, error) {
	runners := make([]models.Agent, 0, len(project.Runners))
	for _, agentID := range project.Runners {
		runner, err := p.store.GetAgent(agentID)
		if err != nil || !runner.HasRole(models.RoleRunner) {
			continue
		}
		runners = append(runners, *runner)
	}

	if len(runners) == 0 {
		return nil, fmt.Errorf("%s: %w", project.Name, ErrNoRunners)
	}
	return runners, nil
}

func (p *Pipeline) claim(release *models.Release, runners []models.Agent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureIdle(release.Project); err != nil {
		return err
	}
	if err := p.store.CreateRelease(release); err != nil {
		return fmt.Errorf("record release: %w", err)
	}

	p.seedTargets(release, runners)
	return nil
}

func (p *Pipeline) ensureIdle(project string) error {
	releases, err := p.store.ListReleasesByProject(project, activeReleaseWindow)
	if err != nil {
		return err
	}

	for _, release := range releases {
		if !release.Status.Done() {
			return fmt.Errorf("%s: %w", project, ErrReleaseInFlight)
		}
	}
	return nil
}

func (p *Pipeline) seedTargets(release *models.Release, runners []models.Agent) {
	for _, runner := range runners {
		status := models.StatusPending
		message := ""
		if !p.link.Online(runner.ID) {
			status = models.StatusSkipped
			message = "agent offline"
		}
		p.store.SaveReleaseTarget(&models.ReleaseTarget{
			ReleaseID: release.ID,
			AgentID:   runner.ID,
			AgentName: runner.Name,
			Status:    status,
			Message:   message,
		})
	}
}
