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
	"fmt"
	"net"
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

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
