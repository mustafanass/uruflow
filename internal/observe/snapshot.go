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

package observe

import (
	"net/url"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/activity"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/secrets"
)

type Snapshot struct {
	Graphs          map[string]ProjectGraphs `json:"graphs"`
	Fleet           Graph                    `json:"fleet_graph"`
	ProjectStates   map[string]string        `json:"project_states"`
	At              time.Time                `json:"at"`
	ServerName      string                   `json:"server_name"`
	Version         string                   `json:"version"`
	Host            *models.Metrics          `json:"host"`
	Agents          []models.Agent           `json:"agents"`
	Projects        []models.Project         `json:"projects"`
	Containers      []models.Container       `json:"containers"`
	Releases        []models.Release         `json:"releases"`
	Alerts          []models.Alert           `json:"alerts"`
	Activity        []activity.Entry         `json:"activity"`
	Problems        []string                 `json:"problems"`
	Registry        string                   `json:"registry"`
	RegistryAddress string                   `json:"registry_address"`
}

func (s Snapshot) Agent(id string) *models.Agent {
	for i := range s.Agents {
		if s.Agents[i].ID == id {
			return &s.Agents[i]
		}
	}
	return nil
}
func (s Snapshot) Project(name string) *models.Project {
	for i := range s.Projects {
		if s.Projects[i].Name == name {
			return &s.Projects[i]
		}
	}
	return nil
}
func (s Snapshot) AgentName(id string) string {
	if a := s.Agent(id); a != nil {
		return a.Name
	}
	if id == "" {
		return "unassigned"
	}
	return id
}
func (s Snapshot) LatestRelease(project string) *models.Release {
	for i := range s.Releases {
		if s.Releases[i].Project == project {
			return &s.Releases[i]
		}
	}
	return nil
}
func (s Snapshot) Fresh(agentID string, at time.Time) bool {
	a := s.Agent(agentID)
	window := 30 * time.Second
	if a != nil && a.Metrics != nil && a.Metrics.MetricsSec > 10 {
		window = time.Duration(a.Metrics.MetricsSec) * 3 * time.Second
	}
	return a != nil && a.Status == models.AgentOnline && !at.IsZero() && at.Unix() > 0 && s.At.Sub(at) >= -5*time.Second && s.At.Sub(at) < window
}
func (s Snapshot) ContainerState(c models.Container, job bool) string {
	if !s.Fresh(c.AgentID, c.ObservedAt) {
		return "unknown"
	}
	if c.State == "exited" && job && c.Details != nil && c.Details.ExitCode == 0 {
		return "completed"
	}
	if c.State != "running" {
		return c.State
	}
	switch c.Health {
	case "healthy":
		return "healthy"
	case "unhealthy":
		return "unhealthy"
	case "starting":
		return "starting"
	}
	return "running"
}
func (s Snapshot) ProjectState(p models.Project) string {
	latest := s.LatestRelease(p.Name)
	found, unknown, bad, ready := 0, 0, 0, 0
	for _, service := range p.ServiceList() {
		for _, runner := range p.Runners {
			matched := false
			for _, c := range s.Containers {
				if c.Project != p.Name || c.Service != service.Name || c.AgentID != runner {
					continue
				}
				matched = true
				found++
				switch s.ContainerState(c, service.EffectiveMode() == models.ServiceModeJob) {
				case "unknown":
					unknown++
				case "healthy", "running", "completed":
					ready++
				default:
					bad++
				}
			}
			if !matched {
				if s.ExpectedState(p, service, runner) == "completed" {
					ready++
					continue
				}
				if latest == nil {
					continue
				}
				if a := s.Agent(runner); a == nil || a.Metrics == nil || !a.Metrics.ContainersAvailable || !s.Fresh(runner, a.Metrics.CollectedAt) {
					unknown++
				} else {
					bad++
				}
			}
		}
	}
	if latest == nil && found == 0 {
		return "undeployed"
	}
	if found == 0 && latest != nil && !latest.Status.Done() {
		return string(latest.Status)
	}
	if p.EffectiveWorkflow() == models.WorkflowBuildOnly && latest != nil && latest.Status == models.StatusSucceeded {
		return "built"
	}
	if bad > 0 {
		return "degraded"
	}
	if unknown > 0 {
		return "unknown"
	}
	if ready > 0 {
		return "running"
	}
	return "undeployed"
}

func RedactProject(p models.Project) models.Project {
	p.GitURL = SafeURL(p.GitURL)
	p.BuildArgs = redactValues(p.BuildArgs)
	p.Runtime.Env = redactValues(p.Runtime.Env)
	p.Runtime.Command = ""
	p.Runtime.CommandExec = nil
	networks := make(map[string]models.NetworkResource, len(p.Networks))
	for name, resource := range p.Networks {
		resource.Options = redactValues(resource.Options)
		resource.Labels = redactValues(resource.Labels)
		networks[name] = resource
	}
	p.Networks = networks
	volumes := make(map[string]models.VolumeResource, len(p.Volumes))
	for name, resource := range p.Volumes {
		resource.Options = redactValues(resource.Options)
		resource.Labels = redactValues(resource.Labels)
		volumes[name] = resource
	}
	p.Volumes = volumes
	services := make([]models.Service, len(p.Services))
	copy(services, p.Services)
	p.Services = services
	for i := range p.Services {
		p.Services[i].Command = ""
		p.Services[i].CommandExec = nil
		p.Services[i].Entrypoint = nil
		p.Services[i].Labels = redactValues(p.Services[i].Labels)
		p.Services[i].Logging.Options = redactValues(p.Services[i].Logging.Options)
		if p.Services[i].Healthcheck != nil {
			check := *p.Services[i].Healthcheck
			check.Command = nil
			check.Path = ""
			p.Services[i].Healthcheck = &check
		}
		p.Services[i].Env = redactValues(p.Services[i].Env)
		p.Services[i].BuildArgs = redactValues(p.Services[i].BuildArgs)
		p.Services[i].GitURL = SafeURL(p.Services[i].GitURL)
	}
	return p
}
func redactValues(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		if strings.HasPrefix(value, "${secret:") && strings.HasSuffix(value, "}") {
			result[key] = value
		} else {
			result[key] = "<set>"
			for _, name := range secrets.References(value) {
				result[key] += " " + secrets.Prefix + name + secrets.Suffix
			}
		}
	}
	return result
}
func SafeURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<source>"
	}
	if parsed.Scheme == "" {
		return raw
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func (s Snapshot) ExpectedState(p models.Project, service models.Service, runner string) string {
	latest := s.LatestRelease(p.Name)
	if latest == nil {
		return "undeployed"
	}
	if service.EffectiveMode() == models.ServiceModeJob && latest.Status == models.StatusSucceeded {
		for _, old := range latest.Spec.ServiceList() {
			if old.Name == service.Name && old.EffectiveMode() == models.ServiceModeJob {
				return "completed"
			}
		}
	}
	a := s.Agent(runner)
	if a == nil || a.Metrics == nil || !a.Metrics.ContainersAvailable || !s.Fresh(runner, a.Metrics.CollectedAt) {
		return "unknown"
	}
	return "missing"
}
