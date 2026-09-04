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

package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	WorkflowBuildDeploy = "build_deploy"
	WorkflowBuildOnly   = "build_only"
	WorkflowDeployOnly  = "deploy_only"

	ServiceModeService = "service"
	ServiceModeJob     = "job"

	DependencyStarted   = "started"
	DependencyHealthy   = "healthy"
	DependencyCompleted = "completed"

	DefaultDeploymentTimeout = 2 * time.Hour
)

func ValidWorkflow(workflow string) bool {
	return workflow == "" || workflow == WorkflowBuildDeploy || workflow == WorkflowBuildOnly || workflow == WorkflowDeployOnly
}

func (p Project) EffectiveWorkflow() string {
	if p.Workflow != "" {
		return p.Workflow
	}
	if len(p.Runners) == 0 {
		return WorkflowBuildOnly
	}
	for _, service := range p.ServiceList() {
		if service.Built() {
			return WorkflowBuildDeploy
		}
	}
	return WorkflowDeployOnly
}

func (p Project) NeedsBuilder() bool {
	workflow := p.EffectiveWorkflow()
	return workflow == WorkflowBuildOnly || workflow == WorkflowBuildDeploy
}

func (p Project) NeedsRunners() bool {
	workflow := p.EffectiveWorkflow()
	return workflow == WorkflowDeployOnly || workflow == WorkflowBuildDeploy
}

func (p Project) EffectiveTimeout() time.Duration {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return DefaultDeploymentTimeout
}

type Dependency struct {
	Service   string `json:"service"`
	Condition string `json:"condition"`
}

type NetworkAttachment struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
}

type NetworkResource struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver,omitempty"`
	External   bool              `json:"external,omitempty"`
	Internal   bool              `json:"internal,omitempty"`
	Attachable bool              `json:"attachable,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type VolumeResource struct {
	Name     string            `json:"name"`
	Driver   string            `json:"driver,omitempty"`
	External bool              `json:"external,omitempty"`
	Options  map[string]string `json:"options,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type ProjectResources struct {
	Networks map[string]NetworkResource `json:"networks,omitempty"`
	Volumes  map[string]VolumeResource  `json:"volumes,omitempty"`
}

type ResourceLimits struct {
	MemoryBytes int64   `json:"memory_bytes,omitempty"`
	CPUs        float64 `json:"cpus,omitempty"`
	PIDs        int64   `json:"pids,omitempty"`
}

type Security struct {
	NoNewPrivileges bool     `json:"no_new_privileges,omitempty"`
	ReadOnlyRootFS  bool     `json:"read_only_rootfs,omitempty"`
	User            string   `json:"user,omitempty"`
	CapAdd          []string `json:"cap_add,omitempty"`
	CapDrop         []string `json:"cap_drop,omitempty"`
}

type LogConfig struct {
	Driver  string            `json:"driver,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

type Job struct {
	Timeout time.Duration `json:"timeout,omitempty"`
}

func ParseMemory(value string) (int64, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(value))
	if trimmed == "" {
		return 0, nil
	}
	multiplier := int64(1)
	units := []struct {
		suffix string
		scale  int64
	}{
		{"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30},
		{"KB", 1000}, {"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000},
		{"K", 1 << 10}, {"M", 1 << 20}, {"G", 1 << 30}, {"B", 1},
	}
	for _, unit := range units {
		suffix, scale := unit.suffix, unit.scale
		if strings.HasSuffix(trimmed, suffix) {
			trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, suffix))
			multiplier = scale
			break
		}
	}
	number, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("memory %q must be a positive size", value)
	}
	bytes := int64(number * float64(multiplier))
	if bytes <= 0 {
		return 0, fmt.Errorf("memory %q is outside the valid range", value)
	}
	return bytes, nil
}

func (s Service) EffectiveMode() string {
	if s.Mode == "" {
		return ServiceModeService
	}
	return s.Mode
}

func (s Service) EffectiveNetworks() []NetworkAttachment {
	if len(s.Networks) > 0 {
		return s.Networks
	}
	if s.Network == "" {
		return nil
	}
	return []NetworkAttachment{{Name: s.Network}}
}

func ValidRestartPolicy(policy string) bool {
	if policy == "" || policy == "no" || policy == "always" || policy == "unless-stopped" || policy == "on-failure" {
		return true
	}
	prefix := "on-failure:"
	if !strings.HasPrefix(policy, prefix) {
		return false
	}
	maximum, err := strconv.Atoi(strings.TrimPrefix(policy, prefix))
	return err == nil && maximum > 0
}

func OrderServices(services []Service) ([]Service, error) {
	byName := make(map[string]Service, len(services))
	for _, service := range services {
		if service.Name == "" && len(services) == 1 {
			return services, nil
		}
		if service.Name == "" {
			return nil, fmt.Errorf("every service in a multi-service project must have a name")
		}
		if _, exists := byName[service.Name]; exists {
			return nil, fmt.Errorf("service %q is declared more than once", service.Name)
		}
		byName[service.Name] = service
	}

	state := make(map[string]uint8, len(services))
	ordered := make([]Service, 0, len(services))
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("service dependency cycle includes %q", name)
		case 2:
			return nil
		}
		service, exists := byName[name]
		if !exists {
			return fmt.Errorf("service %q is not declared", name)
		}
		state[name] = 1
		for _, dependency := range service.DependsOn {
			depends, exists := byName[dependency.Service]
			if !exists {
				return fmt.Errorf("service %q depends on unknown service %q", name, dependency.Service)
			}
			if dependency.Condition == DependencyCompleted && depends.EffectiveMode() != ServiceModeJob {
				return fmt.Errorf("service %q requires %q to complete, but it is not a job", name, dependency.Service)
			}
			if dependency.Condition != DependencyStarted && dependency.Condition != DependencyHealthy && dependency.Condition != DependencyCompleted {
				return fmt.Errorf("service %q dependency %q has invalid condition %q", name, dependency.Service, dependency.Condition)
			}
			if err := visit(dependency.Service); err != nil {
				return err
			}
		}
		state[name] = 2
		ordered = append(ordered, service)
		return nil
	}
	for _, service := range services {
		if err := visit(service.Name); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}
