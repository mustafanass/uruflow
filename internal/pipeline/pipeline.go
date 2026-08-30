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
	"net"
	"slices"
	"strings"
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
	dispatchTimeout = 30 * time.Second
	TagLatest       = "latest"
)

var (
	ErrSecretMissing   = errors.New("secret is not set")
	ErrReleaseInFlight = errors.New("a release is already running for this project")
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
			Branch:    project.Branch,
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
		Branch:      project.Branch,
		Commit:      commit,
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
		GitURL:  project.GitURL,
		Branch:  project.Branch,
		Commit:  commit,
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

func (p *Pipeline) validateProject(project *models.Project) error {
	if !models.ValidResourceName(project.Name) {
		return fmt.Errorf("invalid project name %q", project.Name)
	}
	if !models.ValidWorkflow(project.Workflow) {
		return fmt.Errorf("project %s has invalid workflow %q", project.Name, project.Workflow)
	}
	workflow := project.EffectiveWorkflow()
	if project.NeedsBuilder() && strings.TrimSpace(project.Branch) == "" {
		return fmt.Errorf("project %s build workflow requires a branch", project.Name)
	}
	if !project.NeedsBuilder() && project.Builder != "" {
		return fmt.Errorf("project %s deploy-only workflow must not set a builder", project.Name)
	}
	if !project.NeedsRunners() && len(project.Runners) > 0 {
		return fmt.Errorf("project %s build-only workflow must not set runners", project.Name)
	}
	seen := make(map[string]bool, len(project.Services))
	builtServices := 0
	for key, resource := range project.Networks {
		if !models.ValidResourceName(key) || !models.ValidResourceName(resource.Name) {
			return fmt.Errorf("project %s has invalid network %q", project.Name, key)
		}
	}
	for key, resource := range project.Volumes {
		if !models.ValidResourceName(key) || !models.ValidResourceName(resource.Name) {
			return fmt.Errorf("project %s has invalid volume %q", project.Name, key)
		}
	}
	for _, service := range project.ServiceList() {
		if service.Name != "" && !models.ValidResourceName(service.Name) {
			return fmt.Errorf("invalid service name %q", service.Name)
		}
		if seen[service.Name] {
			return fmt.Errorf("service %q is configured more than once", service.Name)
		}
		seen[service.Name] = true
		if !models.ValidSourcePath(service.BuildFile()) || !models.ValidSourcePath(service.BuildContext()) {
			return fmt.Errorf("service %q build paths must stay inside the source directory", service.Name)
		}
		for _, port := range service.Ports {
			if port.Host < 0 || port.Host > 65535 || port.Container < 1 || port.Container > 65535 ||
				(port.HostIP != "" && net.ParseIP(port.HostIP) == nil) {
				return fmt.Errorf("service %q has an invalid port", service.Name)
			}
		}
		if service.EffectiveMode() != models.ServiceModeService && service.EffectiveMode() != models.ServiceModeJob {
			return fmt.Errorf("service %q has invalid mode %q", service.Name, service.Mode)
		}
		if !models.ValidRestartPolicy(service.Restart) {
			return fmt.Errorf("service %q has invalid restart policy %q", service.Name, service.Restart)
		}
		if service.GitURL != "" && strings.TrimSpace(service.Branch) == "" {
			return fmt.Errorf("service %q source requires a branch", service.Name)
		}
		if service.Built() {
			builtServices++
			if service.GitURL == "" && strings.TrimSpace(project.GitURL) == "" {
				return fmt.Errorf("service %q requires a project or service git URL", service.Name)
			}
		}
		if service.Resources.MemoryBytes < 0 || service.Resources.CPUs < 0 || service.Resources.PIDs < 0 {
			return fmt.Errorf("service %q has invalid resource limits", service.Name)
		}
		for _, network := range service.Networks {
			if _, exists := project.Networks[network.Name]; !exists {
				return fmt.Errorf("service %q uses undeclared network %q", service.Name, network.Name)
			}
		}
		for _, volume := range service.Volumes {
			if volume.Type == "volume" {
				if _, exists := project.Volumes[volume.Source]; !exists {
					return fmt.Errorf("service %q uses undeclared volume %q", service.Name, volume.Source)
				}
			}
		}
		if !service.Built() && !models.ValidDigestReference(service.Image) {
			name := service.Name
			if name == "" {
				name = project.Name
			}
			return fmt.Errorf("service %s image must use repository@sha256:digest", name)
		}
		if err := models.ValidateHealthcheck(service.Healthcheck); err != nil {
			return fmt.Errorf("service %q: %w", service.Name, err)
		}
		if err := models.ValidateLabels(service.Labels); err != nil {
			return fmt.Errorf("service %q: %w", service.Name, err)
		}
	}
	if workflow == models.WorkflowDeployOnly && builtServices > 0 {
		return fmt.Errorf("project %s deploy-only workflow requires immutable images", project.Name)
	}
	if workflow != models.WorkflowDeployOnly && builtServices == 0 {
		return fmt.Errorf("project %s %s workflow has nothing to build", project.Name, workflow)
	}
	if _, err := models.OrderServices(project.ServiceList()); err != nil {
		return fmt.Errorf("project %s: %w", project.Name, err)
	}
	return nil
}

func (p *Pipeline) validateBuildResult(release *models.Release, status ufp.JobStatus) error {
	expected := make(map[string]string)
	first := ""
	for _, service := range release.Spec.ServiceList() {
		if !service.Built() {
			continue
		}
		repository := p.imageRepository(&release.Spec, service)
		expected[service.Name] = repository
		if first == "" {
			first = service.Name
		}
	}
	if len(status.Images) != len(expected) {
		return fmt.Errorf("builder returned %d images, expected %d", len(status.Images), len(expected))
	}
	for service, repository := range expected {
		reference := status.Images[service]
		if !models.ValidDigestReference(reference) || !strings.HasPrefix(reference, repository+"@sha256:") {
			return fmt.Errorf("builder returned an invalid digest reference for service %q", service)
		}
	}
	if status.Image == "" || status.Image != status.Images[first] {
		return fmt.Errorf("builder returned an inconsistent primary image")
	}
	_, digest, _ := strings.Cut(status.Image, "@")
	if status.Digest != digest {
		return fmt.Errorf("builder returned an inconsistent image digest")
	}
	if !models.ValidGitCommit(status.Commit) {
		return fmt.Errorf("builder returned an invalid resolved commit")
	}
	commits := status.Commits
	if len(commits) == 0 {
		commits = make(map[string]string, len(expected))
		for _, service := range release.Spec.ServiceList() {
			if !service.Built() {
				continue
			}
			if service.GitURL != "" {
				return fmt.Errorf("builder did not report the commit for service %q", service.Name)
			}
			commits[service.Name] = status.Commit
		}
	}
	if len(commits) != len(expected) {
		return fmt.Errorf("builder returned %d source commits, expected %d", len(commits), len(expected))
	}
	for _, service := range release.Spec.ServiceList() {
		if !service.Built() {
			continue
		}
		commit := commits[service.Name]
		if !models.ValidGitCommit(commit) {
			return fmt.Errorf("builder returned an invalid commit for service %q", service.Name)
		}
		if service.GitURL == "" && release.Commit != "" && commit != release.Commit && !strings.HasPrefix(commit, release.Commit) {
			return fmt.Errorf("builder resolved service %q commit %s instead of %s", service.Name, commit, release.Commit)
		}
	}
	return nil
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
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureIdle(projectName); err != nil {
		return err
	}

	project, err := p.store.GetProject(projectName)
	if err != nil {
		return fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
	}

	runners, err := p.resolveRunners(project)
	if err != nil {
		return err
	}
	var failures []error
	for _, runner := range runners {
		if err := p.dispatch(runner.ID, ufp.MethodReleaseStop, ufp.ProjectRef{Project: project.Name}); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", runner.Name, err))
		}
	}
	return errors.Join(failures...)
}

func (p *Pipeline) Remove(projectName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureIdle(projectName); err != nil {
		return err
	}

	project, err := p.store.GetProject(projectName)
	if err != nil {
		return fmt.Errorf("%s: %w", projectName, ErrProjectNotFound)
	}

	runners, err := p.resolveRunners(project)
	if err != nil {
		return err
	}
	var failures []error
	for _, runner := range runners {
		if err := p.dispatch(runner.ID, ufp.MethodReleaseRemove, ufp.ProjectRef{Project: project.Name}); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", runner.Name, err))
		}
	}
	return errors.Join(failures...)
}

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

	if err := p.ensureIdle(release.Project); err != nil {
		return err
	}
	targets := p.releaseTargets(release, runners)
	if err := p.store.ClaimRelease(release, targets); err != nil {
		return fmt.Errorf("record release: %w", err)
	}
	return nil
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
