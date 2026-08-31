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

package projects

import (
	"fmt"
	"strings"
)

func Interpolate(value string, variables map[string]string) (string, error) {
	const escapedDollar = "\x00uruflow-dollar\x00"
	value = strings.ReplaceAll(value, "$$", escapedDollar)
	var output strings.Builder
	for {
		start := strings.Index(value, "${")
		if start < 0 {
			output.WriteString(value)
			break
		}
		output.WriteString(value[:start])
		end := strings.IndexByte(value[start+2:], '}')
		if end < 0 {
			return "", fmt.Errorf("unterminated variable expression")
		}
		end += start + 2
		expression := value[start+2 : end]
		original := value[start : end+1]
		if strings.HasPrefix(expression, "secret:") {
			output.WriteString(original)
			value = value[end+1:]
			continue
		}

		name, operator, operand := expression, "", ""
		if index := strings.Index(expression, ":-"); index >= 0 {
			name, operator, operand = expression[:index], ":-", expression[index+2:]
		} else if index := strings.Index(expression, ":?"); index >= 0 {
			name, operator, operand = expression[:index], ":?", expression[index+2:]
		}
		if name == "" {
			return "", fmt.Errorf("variable name is empty")
		}
		if !validVariableName(name) {
			return "", fmt.Errorf("variable name %q is invalid", name)
		}
		resolved := variables[name]
		switch {
		case resolved != "":
			output.WriteString(resolved)
		case operator == ":-":
			output.WriteString(operand)
		case operator == ":?":
			if operand == "" {
				operand = name + " must be set"
			}
			return "", fmt.Errorf("%s", operand)
		default:
			output.WriteString("")
		}
		value = value[end+1:]
	}
	return strings.ReplaceAll(output.String(), escapedDollar, "$"), nil
}

func validVariableName(name string) bool {
	for index, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' || (index > 0 && char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return name != ""
}

func interpolateMap(values map[string]string, variables map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return values, nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		expanded, err := Interpolate(value, variables)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		result[key] = expanded
	}
	return result, nil
}

func resolveVariables(values map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return values, nil
	}
	current := make(map[string]string, len(values))
	for key, value := range values {
		current[key] = value
	}
	seen := make(map[string]bool)
	for pass := 0; pass <= len(values); pass++ {
		fingerprint := fmt.Sprint(current)
		if seen[fingerprint] {
			return nil, fmt.Errorf("environment variable interpolation contains a cycle")
		}
		seen[fingerprint] = true
		next, err := interpolateMap(current, current)
		if err != nil {
			return nil, err
		}
		stable := true
		for key := range current {
			if current[key] != next[key] {
				stable = false
				break
			}
		}
		if stable {
			for _, value := range next {
				for rest := value; ; {
					start := strings.Index(rest, "${")
					if start < 0 {
						break
					}
					end := strings.IndexByte(rest[start+2:], '}')
					if end < 0 {
						break
					}
					expression := rest[start+2 : start+2+end]
					if !strings.HasPrefix(expression, "secret:") {
						return nil, fmt.Errorf("environment variable interpolation contains a cycle")
					}
					rest = rest[start+3+end:]
				}
			}
			return next, nil
		}
		current = next
	}
	return nil, fmt.Errorf("environment variable interpolation did not converge")
}

func interpolateService(service Service, variables map[string]string) (Service, error) {
	var err error
	fields := []*string{
		&service.Image, &service.Git, &service.Branch, &service.Dockerfile, &service.Context,
		&service.Network, &service.Restart, &service.Mode, &service.Timeout, &service.Command.Shell,
		&service.Security.User, &service.Logging.Driver,
	}
	for _, field := range fields {
		*field, err = Interpolate(*field, variables)
		if err != nil {
			return Service{}, err
		}
	}
	for index := range service.Entrypoint {
		service.Entrypoint[index], err = Interpolate(service.Entrypoint[index], variables)
		if err != nil {
			return Service{}, err
		}
	}
	for index := range service.Command.Exec {
		service.Command.Exec[index], err = Interpolate(service.Command.Exec[index], variables)
		if err != nil {
			return Service{}, err
		}
	}
	for index := range service.Ports {
		service.Ports[index], err = Interpolate(service.Ports[index], variables)
		if err != nil {
			return Service{}, err
		}
	}
	for index := range service.Volumes {
		service.Volumes[index], err = Interpolate(service.Volumes[index], variables)
		if err != nil {
			return Service{}, err
		}
	}
	for index := range service.Mounts {
		service.Mounts[index].Source, err = Interpolate(service.Mounts[index].Source, variables)
		if err != nil {
			return Service{}, err
		}
		service.Mounts[index].Target, err = Interpolate(service.Mounts[index].Target, variables)
		if err != nil {
			return Service{}, err
		}
	}
	for key, attachment := range service.Networks {
		for index := range attachment.Aliases {
			attachment.Aliases[index], err = Interpolate(attachment.Aliases[index], variables)
			if err != nil {
				return Service{}, err
			}
		}
		service.Networks[key] = attachment
	}
	for index := range service.Security.CapAdd {
		service.Security.CapAdd[index], err = Interpolate(service.Security.CapAdd[index], variables)
		if err != nil {
			return Service{}, err
		}
	}
	for index := range service.Security.CapDrop {
		service.Security.CapDrop[index], err = Interpolate(service.Security.CapDrop[index], variables)
		if err != nil {
			return Service{}, err
		}
	}
	service.BuildArgs, err = interpolateMap(service.BuildArgs, variables)
	if err != nil {
		return Service{}, err
	}
	service.Env, err = interpolateMap(service.Env, variables)
	if err != nil {
		return Service{}, err
	}
	service.Labels, err = interpolateMap(service.Labels, variables)
	if err != nil {
		return Service{}, err
	}
	service.Logging.Options, err = interpolateMap(service.Logging.Options, variables)
	if err != nil {
		return Service{}, err
	}
	if service.Healthcheck != nil {
		service.Healthcheck.Scheme, err = Interpolate(service.Healthcheck.Scheme, variables)
		if err != nil {
			return Service{}, err
		}
		service.Healthcheck.Path, err = Interpolate(service.Healthcheck.Path, variables)
		if err != nil {
			return Service{}, err
		}
		service.Healthcheck.Command.Shell, err = Interpolate(service.Healthcheck.Command.Shell, variables)
		if err != nil {
			return Service{}, err
		}
		for index := range service.Healthcheck.Command.Exec {
			service.Healthcheck.Command.Exec[index], err = Interpolate(service.Healthcheck.Command.Exec[index], variables)
			if err != nil {
				return Service{}, err
			}
		}
	}
	return service, nil
}

func interpolateEnvironmentRuntime(environment Environment, variables map[string]string) (Environment, error) {
	var err error
	fields := []*string{&environment.Network, &environment.Restart, &environment.Command.Shell}
	for _, field := range fields {
		*field, err = Interpolate(*field, variables)
		if err != nil {
			return Environment{}, err
		}
	}
	for index := range environment.Command.Exec {
		environment.Command.Exec[index], err = Interpolate(environment.Command.Exec[index], variables)
		if err != nil {
			return Environment{}, err
		}
	}
	for index := range environment.Ports {
		environment.Ports[index], err = Interpolate(environment.Ports[index], variables)
		if err != nil {
			return Environment{}, err
		}
	}
	for index := range environment.Volumes {
		environment.Volumes[index], err = Interpolate(environment.Volumes[index], variables)
		if err != nil {
			return Environment{}, err
		}
	}
	return environment, nil
}

func interpolateNetworks(resources map[string]Network, variables map[string]string) (map[string]Network, error) {
	for key, resource := range resources {
		var err error
		resource.Name, err = Interpolate(resource.Name, variables)
		if err != nil {
			return nil, fmt.Errorf("%s.name: %w", key, err)
		}
		resource.Driver, err = Interpolate(resource.Driver, variables)
		if err != nil {
			return nil, fmt.Errorf("%s.driver: %w", key, err)
		}
		resource.Options, err = interpolateMap(resource.Options, variables)
		if err != nil {
			return nil, fmt.Errorf("%s.options: %w", key, err)
		}
		resource.Labels, err = interpolateMap(resource.Labels, variables)
		if err != nil {
			return nil, fmt.Errorf("%s.labels: %w", key, err)
		}
		resources[key] = resource
	}
	return resources, nil
}

func interpolateVolumes(resources map[string]VolumeDefinition, variables map[string]string) (map[string]VolumeDefinition, error) {
	for key, resource := range resources {
		var err error
		resource.Name, err = Interpolate(resource.Name, variables)
		if err != nil {
			return nil, fmt.Errorf("%s.name: %w", key, err)
		}
		resource.Driver, err = Interpolate(resource.Driver, variables)
		if err != nil {
			return nil, fmt.Errorf("%s.driver: %w", key, err)
		}
		resource.Options, err = interpolateMap(resource.Options, variables)
		if err != nil {
			return nil, fmt.Errorf("%s.options: %w", key, err)
		}
		resource.Labels, err = interpolateMap(resource.Labels, variables)
		if err != nil {
			return nil, fmt.Errorf("%s.labels: %w", key, err)
		}
		resources[key] = resource
	}
	return resources, nil
}
