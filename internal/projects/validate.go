/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 * This file is part of uruflow and is licensed under the MIT License.
 */

package projects

import (
	"errors"
	"fmt"

	"github.com/mustafanass/uruflow/internal/models"
)

func ValidateEnvironmentYAML(content string) error {
	return ValidateEnvironmentYAMLWithVariables(content, nil)
}

func ValidateEnvironmentYAMLWithVariables(content string, variables map[string]string) error {
	var environment Environment
	if err := decodeStrict([]byte(content), &environment); err != nil {
		return err
	}
	resolved, err := resolveVariables(variables)
	if err != nil {
		return fmt.Errorf("environment: %w", err)
	}
	environment, err = interpolateEnvironmentRuntime(environment, resolved)
	if err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	if _, err := models.ParsePorts(environment.Ports); err != nil {
		return err
	}
	if _, err := models.ParseVolumes(environment.Volumes); err != nil {
		return err
	}
	if !models.ValidRestartPolicy(environment.Restart) {
		return fmt.Errorf("invalid restart policy %q", environment.Restart)
	}
	services, err := buildServices(environment.Services, resolved)
	if err != nil {
		return err
	}
	if !models.ValidWorkflow(environment.Workflow) {
		return fmt.Errorf("workflow %q is not supported", environment.Workflow)
	}
	if environment.Workflow != "" {
		if err := validateExplicitWorkflow(environment, services); err != nil {
			return err
		}
	}
	if _, err := models.OrderServices(services); err != nil {
		return err
	}
	if len(environment.Networks) > 0 && len(environment.Resources.Networks) > 0 {
		return errors.New("set resource networks under resources.networks only")
	}
	if len(environment.VolumeResources) > 0 && len(environment.Resources.Volumes) > 0 {
		return errors.New("set resource volumes under resources.volumes only")
	}
	declaredNetworks, err := interpolateNetworks(environmentNetworks(environment), resolved)
	if err != nil {
		return fmt.Errorf("networks: %w", err)
	}
	declaredVolumes, err := interpolateVolumes(environmentVolumes(environment), resolved)
	if err != nil {
		return fmt.Errorf("volumes: %w", err)
	}
	networks, err := buildNetworks("", declaredNetworks)
	if err != nil {
		return err
	}
	volumes, err := buildVolumeResources("", declaredVolumes)
	if err != nil {
		return err
	}
	return validateServiceResources(services, networks, volumes)
}

func validateExplicitWorkflow(environment Environment, services []models.Service) error {
	probe := models.Project{Workflow: environment.Workflow, Services: services, Runners: environment.Runners}
	workflow := probe.EffectiveWorkflow()
	built := false
	for _, service := range probe.ServiceList() {
		built = built || service.Built()
	}
	if workflow == models.WorkflowDeployOnly && built {
		return errors.New("deploy_only requires immutable images for every service")
	}
	if workflow != models.WorkflowDeployOnly && !built {
		return fmt.Errorf("%s requires at least one source-built service", workflow)
	}
	if probe.NeedsBuilder() && (environment.Branch == "" || environment.Builder == "") {
		return errors.New("build workflows require branch and builder")
	}
	if !probe.NeedsBuilder() && environment.Builder != "" {
		return errors.New("deploy_only must not set a builder")
	}
	if probe.NeedsRunners() && len(environment.Runners) == 0 {
		return errors.New("deployment workflows require at least one runner")
	}
	if !probe.NeedsRunners() && len(environment.Runners) > 0 {
		return errors.New("build_only must not set runners")
	}
	return nil
}

func environmentNetworks(environment Environment) map[string]Network {
	if len(environment.Resources.Networks) > 0 {
		return environment.Resources.Networks
	}
	return environment.Networks
}

func environmentVolumes(environment Environment) map[string]VolumeDefinition {
	if len(environment.Resources.Volumes) > 0 {
		return environment.Resources.Volumes
	}
	return environment.VolumeResources
}

func mergeEnv(layers ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, layer := range layers {
		for key, value := range layer {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}
