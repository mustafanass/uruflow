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

	"github.com/mustafanass/uruflow/internal/models"
	"gopkg.in/yaml.v3"
)

const (
	DefaultsFile  = "defaults.yaml"
	EnvSuffix     = ".env"
	YAMLSuffix    = ".yaml"
	NameSeparator = "-"
)

type Defaults struct {
	Env map[string]string `yaml:"env,omitempty"`
}

type Service struct {
	Image       string                       `yaml:"image,omitempty"`
	Dockerfile  string                       `yaml:"dockerfile,omitempty"`
	Context     string                       `yaml:"context,omitempty"`
	BuildArgs   map[string]string            `yaml:"build_args,omitempty"`
	Git         string                       `yaml:"git,omitempty"`
	Branch      string                       `yaml:"branch,omitempty"`
	Entrypoint  []string                     `yaml:"entrypoint,omitempty"`
	Command     Command                      `yaml:"command,omitempty"`
	Ports       []string                     `yaml:"ports,omitempty"`
	Volumes     []string                     `yaml:"volumes,omitempty"`
	Mounts      []Mount                      `yaml:"mounts,omitempty"`
	Env         map[string]string            `yaml:"env,omitempty"`
	Network     string                       `yaml:"network,omitempty"`
	Networks    map[string]NetworkAttachment `yaml:"networks,omitempty"`
	Restart     string                       `yaml:"restart,omitempty"`
	Mode        string                       `yaml:"mode,omitempty"`
	DependsOn   map[string]string            `yaml:"depends_on,omitempty"`
	Resources   Resources                    `yaml:"resources,omitempty"`
	Security    Security                     `yaml:"security,omitempty"`
	Logging     Logging                      `yaml:"logging,omitempty"`
	Timeout     string                       `yaml:"timeout,omitempty"`
	Healthcheck *Healthcheck                 `yaml:"healthcheck,omitempty"`
	Labels      map[string]string            `yaml:"labels,omitempty"`
}

type Healthcheck struct {
	Type        string  `yaml:"type"`
	Scheme      string  `yaml:"scheme,omitempty"`
	Path        string  `yaml:"path,omitempty"`
	Port        int     `yaml:"port,omitempty"`
	Interval    string  `yaml:"interval,omitempty"`
	Timeout     string  `yaml:"timeout,omitempty"`
	Retries     *int    `yaml:"retries,omitempty"`
	StableFor   string  `yaml:"stable_for,omitempty"`
	Command     Command `yaml:"command,omitempty"`
	StartPeriod string  `yaml:"start_period,omitempty"`
}

type Command struct {
	Shell string
	Exec  []string
}

func (c *Command) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return node.Decode(&c.Shell)
	case yaml.SequenceNode:
		return node.Decode(&c.Exec)
	default:
		return fmt.Errorf("command must be a string or string list")
	}
}

func (c Command) MarshalYAML() (any, error) {
	if len(c.Exec) > 0 {
		return c.Exec, nil
	}
	if c.Shell == "" {
		return nil, nil
	}
	return c.Shell, nil
}

type NetworkAttachment struct {
	Aliases []string `yaml:"aliases,omitempty"`
}

type Network struct {
	Name       string            `yaml:"name,omitempty"`
	Driver     string            `yaml:"driver,omitempty"`
	External   bool              `yaml:"external,omitempty"`
	Internal   bool              `yaml:"internal,omitempty"`
	Attachable bool              `yaml:"attachable,omitempty"`
	Options    map[string]string `yaml:"options,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty"`
}

type VolumeDefinition struct {
	Name     string            `yaml:"name,omitempty"`
	Driver   string            `yaml:"driver,omitempty"`
	External bool              `yaml:"external,omitempty"`
	Options  map[string]string `yaml:"options,omitempty"`
	Labels   map[string]string `yaml:"labels,omitempty"`
}

type ResourceDefinitions struct {
	Networks map[string]Network          `yaml:"networks,omitempty"`
	Volumes  map[string]VolumeDefinition `yaml:"volumes,omitempty"`
}

type Mount struct {
	Type     string      `yaml:"type"`
	Source   string      `yaml:"source,omitempty"`
	Target   string      `yaml:"target"`
	ReadOnly bool        `yaml:"read_only,omitempty"`
	Bind     BindOptions `yaml:"bind,omitempty"`
}

type BindOptions struct {
	CreateHostPath bool `yaml:"create_host_path,omitempty"`
}

type Resources struct {
	Memory string  `yaml:"memory,omitempty"`
	CPUs   float64 `yaml:"cpus,omitempty"`
	PIDs   int64   `yaml:"pids,omitempty"`
}

type Security struct {
	NoNewPrivileges bool     `yaml:"no_new_privileges,omitempty"`
	ReadOnlyRootFS  bool     `yaml:"read_only_rootfs,omitempty"`
	User            string   `yaml:"user,omitempty"`
	CapAdd          []string `yaml:"cap_add,omitempty"`
	CapDrop         []string `yaml:"cap_drop,omitempty"`
}

type Logging struct {
	Driver  string            `yaml:"driver,omitempty"`
	Options map[string]string `yaml:"options,omitempty"`
}

type Environment struct {
	Workflow        string                      `yaml:"workflow,omitempty"`
	Builder         string                      `yaml:"builder"`
	Runners         []string                    `yaml:"runners"`
	Ports           []string                    `yaml:"ports,omitempty"`
	Volumes         []string                    `yaml:"volumes,omitempty"`
	Network         string                      `yaml:"network,omitempty"`
	Restart         string                      `yaml:"restart,omitempty"`
	Command         Command                     `yaml:"command,omitempty"`
	Env             map[string]string           `yaml:"env,omitempty"`
	Services        map[string]Service          `yaml:"services,omitempty"`
	Networks        map[string]Network          `yaml:"networks,omitempty"`
	VolumeResources map[string]VolumeDefinition `yaml:"volume_resources,omitempty"`
	Resources       ResourceDefinitions         `yaml:"resources,omitempty"`
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
