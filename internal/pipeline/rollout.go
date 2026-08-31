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
	"github.com/mustafanass/uruflow/pkg/logger"
)

const dispatchTimeout = 30 * time.Second

func (p *Pipeline) rollout(release *models.Release, project *models.Project, runners []models.Agent) {
	request, err := p.releaseRequest(release, project)
	if err != nil {
		p.mu.Lock()
		p.failRelease(release, err.Error())
		p.mu.Unlock()
		return
	}

	for _, runner := range runners {
		if !p.link.Online(runner.ID) {
			p.mu.Lock()
			if err := p.finishTarget(release.ID, runner.ID, runner.Name, models.StatusSkipped, "agent offline"); err != nil {
				logger.Error("[PIPELINE] update target %s/%s: %v", release.ID, runner.ID, err)
			}
			p.mu.Unlock()
			continue
		}
		if err := p.dispatch(runner.ID, ufp.MethodReleaseRun, *request); err != nil {
			p.mu.Lock()
			if saveErr := p.finishTarget(release.ID, runner.ID, runner.Name, models.StatusFailed, err.Error()); saveErr != nil {
				logger.Error("[PIPELINE] update target %s/%s: %v", release.ID, runner.ID, saveErr)
			}
			p.mu.Unlock()
		}
	}

	p.mu.Lock()
	p.settle(release.ID)
	p.mu.Unlock()
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
			GitURL:     service.GitURL,
			Branch:     service.Branch,
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

	request := &ufp.ReleaseRequest{JobID: release.ID, Project: project.Name, Networks: make(map[string]ufp.NetworkResource), Volumes: make(map[string]ufp.VolumeResource)}
	for key, resource := range project.Networks {
		request.Networks[key] = ufp.NetworkResource{Name: resource.Name, Driver: resource.Driver, External: resource.External, Internal: resource.Internal, Attachable: resource.Attachable, Options: resource.Options, Labels: resource.Labels}
	}
	for key, resource := range project.Volumes {
		request.Volumes[key] = ufp.VolumeResource{Name: resource.Name, Driver: resource.Driver, External: resource.External, Options: resource.Options, Labels: resource.Labels}
	}
	if len(request.Networks) == 0 {
		request.Networks = nil
	}
	if len(request.Volumes) == 0 {
		request.Volumes = nil
	}

	ordered, err := models.OrderServices(project.ServiceList())
	if err != nil {
		return nil, err
	}
	for _, service := range ordered {
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
			Name:        service.Name,
			Image:       image,
			Env:         env,
			Network:     service.Network,
			Restart:     service.RestartPolicy(),
			Command:     service.Command,
			CommandExec: service.CommandExec,
			Entrypoint:  service.Entrypoint,
			Mode:        service.EffectiveMode(),
			JobTimeout:  service.Job.Timeout,
			Labels:      service.Labels,
		}
		for _, network := range service.EffectiveNetworks() {
			aliases := append([]string(nil), network.Aliases...)
			if service.Name != "" && !slices.Contains(aliases, service.Name) {
				aliases = append(aliases, service.Name)
			}
			spec.Networks = append(spec.Networks, ufp.NetworkAttachment{Name: network.Name, Aliases: aliases})
		}
		for _, dependency := range service.DependsOn {
			spec.DependsOn = append(spec.DependsOn, ufp.Dependency{Service: dependency.Service, Condition: dependency.Condition})
		}
		spec.Resources = ufp.ResourceLimits{MemoryBytes: service.Resources.MemoryBytes, CPUs: service.Resources.CPUs, PIDs: service.Resources.PIDs}
		spec.Security = ufp.SecuritySpec{NoNewPrivileges: service.Security.NoNewPrivileges, ReadOnlyRootFS: service.Security.ReadOnlyRootFS, User: service.Security.User, CapAdd: service.Security.CapAdd, CapDrop: service.Security.CapDrop}
		spec.Logging = ufp.LogConfig{Driver: service.Logging.Driver, Options: service.Logging.Options}
		if service.Healthcheck != nil {
			spec.Healthcheck = &ufp.HealthcheckSpec{
				Type: service.Healthcheck.Type, Scheme: service.Healthcheck.Scheme,
				Path: service.Healthcheck.Path, Port: service.Healthcheck.Port,
				Interval: service.Healthcheck.Interval, Timeout: service.Healthcheck.Timeout,
				Retries: service.Healthcheck.Retries, StableFor: service.Healthcheck.StableFor,
				Command: service.Healthcheck.Command, StartPeriod: service.Healthcheck.StartPeriod,
			}
		}
		for _, port := range service.Ports {
			spec.Ports = append(spec.Ports, ufp.PortBinding{
				HostIP: port.HostIP, Host: port.Host, Container: port.Container, Protocol: port.Protocol,
			})
		}
		for _, volume := range service.Volumes {
			spec.Volumes = append(spec.Volumes, ufp.VolumeBinding{
				Type: volume.Type, Source: volume.Source, Target: volume.Target, ReadOnly: volume.ReadOnly, CreateHostPath: volume.CreateHostPath,
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
	seen := make(map[string]bool, len(project.Runners))
	for _, agentID := range project.Runners {
		if seen[agentID] {
			return nil, fmt.Errorf("%s: runner %s is configured more than once", project.Name, agentID)
		}
		seen[agentID] = true
		runner, err := p.store.GetAgent(agentID)
		if err != nil {
			return nil, fmt.Errorf("%s: configured runner %s was not found", project.Name, agentID)
		}
		if !runner.HasRole(models.RoleRunner) {
			return nil, fmt.Errorf("%s: %w", runner.Name, ErrNotRunner)
		}
		if !p.link.Online(runner.ID) {
			return nil, fmt.Errorf("%s: %w", runner.Name, ErrRunnerOffline)
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

	if _, exists := p.busy[release.Project]; exists {
		return fmt.Errorf("%s: %w", release.Project, ErrProjectBusy)
	}
	if err := p.ensureIdle(release.Project); err != nil {
		return err
	}
	targets := p.releaseTargets(release, runners)
	if err := p.store.ClaimRelease(release, targets); err != nil {
		return fmt.Errorf("record release: %w", err)
	}
	p.publishRelease(release)
	return nil
}

func (p *Pipeline) reserve(project string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.busy[project]; exists {
		return fmt.Errorf("%s: %w", project, ErrProjectBusy)
	}
	if err := p.ensureIdle(project); err != nil {
		return err
	}
	if p.busy == nil {
		p.busy = make(map[string]struct{})
	}
	p.busy[project] = struct{}{}
	return nil
}

func (p *Pipeline) releaseReservation(project string) {
	p.mu.Lock()
	delete(p.busy, project)
	p.mu.Unlock()
}

func (p *Pipeline) ensureIdle(project string) error {
	active, err := p.store.ProjectHasActiveRelease(project)
	if err != nil {
		return err
	}
	if active {
		return fmt.Errorf("%s: %w", project, ErrReleaseInFlight)
	}
	return nil
}

func (p *Pipeline) releaseTargets(release *models.Release, runners []models.Agent) []models.ReleaseTarget {
	targets := make([]models.ReleaseTarget, 0, len(runners))
	for _, runner := range runners {
		targets = append(targets, models.ReleaseTarget{
			ReleaseID: release.ID,
			AgentID:   runner.ID,
			AgentName: runner.Name,
			Status:    models.StatusPending,
		})
	}
	return targets
}
