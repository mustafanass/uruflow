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

import "github.com/urustack/uruflow/internal/models"

const (
	ProjectFile   = "project.yaml"
	DefaultsFile  = "defaults.yaml"
	EnvSuffix     = ".env"
	YamlSuffix    = ".yaml"
	NameSeparator = "-"
)

type Defaults struct {
	Env map[string]string `yaml:"env,omitempty"`
}

type Definition struct {
	Name       string            `yaml:"name,omitempty"`
	Git        string            `yaml:"git"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Context    string            `yaml:"context,omitempty"`
	BuildArgs  map[string]string `yaml:"build_args,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
}

type Service struct {
	Image      string            `yaml:"image,omitempty"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Context    string            `yaml:"context,omitempty"`
	BuildArgs  map[string]string `yaml:"build_args,omitempty"`
	Command    string            `yaml:"command,omitempty"`
	Ports      []string          `yaml:"ports,omitempty"`
	Volumes    []string          `yaml:"volumes,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	Network    string            `yaml:"network,omitempty"`
	Restart    string            `yaml:"restart,omitempty"`
}

type Environment struct {
	Branch     string             `yaml:"branch"`
	Builder    string             `yaml:"builder"`
	Runners    []string           `yaml:"runners"`
	AutoDeploy *bool              `yaml:"auto_deploy,omitempty"`
	Ports      []string           `yaml:"ports,omitempty"`
	Volumes    []string           `yaml:"volumes,omitempty"`
	Network    string             `yaml:"network,omitempty"`
	Restart    string             `yaml:"restart,omitempty"`
	Command    string             `yaml:"command,omitempty"`
	Env        map[string]string  `yaml:"env,omitempty"`
	Services   map[string]Service `yaml:"services,omitempty"`
}

type Problem struct {
	Path   string
	Reason error
}

func (p Problem) Error() string {
	return p.Path + ": " + p.Reason.Error()
}

type Result struct {
	Projects []models.Project
	Problems []Problem
}
