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
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/ufp"
)

type Role = ufp.Role

const (
	RoleBuilder = ufp.RoleBuilder
	RoleRunner  = ufp.RoleRunner
)

type AgentStatus string

const (
	AgentOnline  AgentStatus = "online"
	AgentOffline AgentStatus = "offline"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusBuilding  Status = "building"
	StatusReleasing Status = "releasing"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
)

type Trigger string

const (
	TriggerWebhook  Trigger = "webhook"
	TriggerManual   Trigger = "manual"
	TriggerRollback Trigger = "rollback"
)

type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Agent struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Key          string      `json:"-"`
	Roles        []Role      `json:"roles"`
	Host         string      `json:"host"`
	Hostname     string      `json:"hostname"`
	Version      string      `json:"version"`
	Platform     string      `json:"platform"`
	Status       AgentStatus `json:"status"`
	Metrics      *Metrics    `json:"metrics,omitempty"`
	LastSeen     time.Time   `json:"last_seen"`
	RegisteredAt time.Time   `json:"registered_at"`
}

type Metrics struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryPercent float64   `json:"memory_percent"`
	MemoryUsed    uint64    `json:"memory_used"`
	MemoryTotal   uint64    `json:"memory_total"`
	DiskPercent   float64   `json:"disk_percent"`
	DiskUsed      uint64    `json:"disk_used"`
	DiskTotal     uint64    `json:"disk_total"`
	LoadAvg       []float64 `json:"load_avg,omitempty"`
	Uptime        int64     `json:"uptime"`
}

type Project struct {
	Name       string            `json:"name"`
	GitURL     string            `json:"git_url"`
	Branch     string            `json:"branch"`
	Dockerfile string            `json:"dockerfile"`
	Context    string            `json:"context"`
	BuildArgs  map[string]string `json:"build_args,omitempty"`
	Builder    string            `json:"builder"`
	Runners    []string          `json:"runners"`
	AutoDeploy bool              `json:"auto_deploy"`
	Runtime    Runtime           `json:"runtime"`
	Services   []Service         `json:"services,omitempty"`
	Env        string            `json:"env,omitempty"`
	Source     string            `json:"source,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

type Service struct {
	Name       string            `json:"name"`
	Image      string            `json:"image,omitempty"`
	Dockerfile string            `json:"dockerfile,omitempty"`
	Context    string            `json:"context,omitempty"`
	BuildArgs  map[string]string `json:"build_args,omitempty"`
	Command    string            `json:"command,omitempty"`
	Ports      []Port            `json:"ports,omitempty"`
	Volumes    []Volume          `json:"volumes,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Network    string            `json:"network,omitempty"`
	Restart    string            `json:"restart,omitempty"`
}

type Runtime struct {
	Ports   []Port            `json:"ports,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Volumes []Volume          `json:"volumes,omitempty"`
	Network string            `json:"network,omitempty"`
	Restart string            `json:"restart,omitempty"`
	Command string            `json:"command,omitempty"`
}

type Port struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol,omitempty"`
}

type Volume struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type Release struct {
	ID          string            `json:"id"`
	Project     string            `json:"project"`
	Branch      string            `json:"branch"`
	Commit      string            `json:"commit"`
	Image       string            `json:"image"`
	Images      map[string]string `json:"images,omitempty"`
	Digest      string            `json:"digest"`
	Status      Status            `json:"status"`
	Builder     string            `json:"builder"`
	BuilderName string            `json:"builder_name"`
	Trigger     Trigger           `json:"trigger"`
	Message     string            `json:"message,omitempty"`
	Targets     []ReleaseTarget   `json:"targets,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	EndedAt     *time.Time        `json:"ended_at,omitempty"`
	Duration    int64             `json:"duration"`
}

type ReleaseTarget struct {
	ReleaseID string     `json:"release_id"`
	AgentID   string     `json:"agent_id"`
	AgentName string     `json:"agent_name"`
	Status    Status     `json:"status"`
	Message   string     `json:"message,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

type LogLine struct {
	ID        int64     `json:"id"`
	ReleaseID string    `json:"release_id"`
	Stage     string    `json:"stage"`
	AgentName string    `json:"agent_name"`
	Stream    string    `json:"stream"`
	Line      string    `json:"line"`
	Timestamp time.Time `json:"timestamp"`
}

type Container struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	Name         string    `json:"name"`
	Project      string    `json:"project"`
	Service      string    `json:"service,omitempty"`
	Image        string    `json:"image"`
	State        string    `json:"state"`
	Health       string    `json:"health"`
	CPUPercent   float64   `json:"cpu_percent"`
	MemoryUsage  uint64    `json:"memory_usage"`
	MemoryLimit  uint64    `json:"memory_limit"`
	NetworkRx    uint64    `json:"network_rx"`
	NetworkTx    uint64    `json:"network_tx"`
	RestartCount int       `json:"restart_count"`
	StartedAt    time.Time `json:"started_at"`
}

type Image struct {
	Repository string    `json:"repository"`
	Tag        string    `json:"tag"`
	Digest     string    `json:"digest"`
	Size       int64     `json:"size"`
	CreatedAt  time.Time `json:"created_at"`
}

type Secret struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Alert struct {
	ID         string     `json:"id"`
	AgentID    string     `json:"agent_id"`
	AgentName  string     `json:"agent_name"`
	Type       string     `json:"type"`
	Message    string     `json:"message"`
	Severity   Severity   `json:"severity"`
	Resolved   bool       `json:"resolved"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

func (s Status) Done() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusSkipped
}

func (a *Agent) HasRole(role Role) bool {
	return ufp.HasRole(a.Roles, role)
}

func (p *Project) BuildContext() string {
	if p.Context == "" {
		return "."
	}
	return p.Context
}

func (p *Project) BuildFile() string {
	if p.Dockerfile == "" {
		return "Dockerfile"
	}
	return p.Dockerfile
}

func (p *Project) Managed() bool {
	return p.Source != ""
}

func (p *Project) Base() string {
	if p.Env == "" {
		return p.Name
	}
	return strings.TrimSuffix(p.Name, "-"+p.Env)
}

func (s Service) Built() bool {
	return s.Image == ""
}

func (s Service) BuildFile() string {
	if s.Dockerfile == "" {
		return "Dockerfile"
	}
	return s.Dockerfile
}

func (s Service) BuildContext() string {
	if s.Context == "" {
		return "."
	}
	return s.Context
}

func (s Service) RestartPolicy() string {
	if s.Restart == "" {
		return "unless-stopped"
	}
	return s.Restart
}

func (p *Project) MultiService() bool {
	return len(p.Services) > 0
}

func (p *Project) ServiceList() []Service {
	if p.MultiService() {
		return p.Services
	}

	return []Service{{
		Name:       "",
		Dockerfile: p.Dockerfile,
		Context:    p.Context,
		BuildArgs:  p.BuildArgs,
		Command:    p.Runtime.Command,
		Ports:      p.Runtime.Ports,
		Volumes:    p.Runtime.Volumes,
		Env:        p.Runtime.Env,
		Network:    p.Runtime.Network,
		Restart:    p.Runtime.Restart,
	}}
}

func (p *Project) ServiceEnv(service Service) map[string]string {
	merged := make(map[string]string, len(p.Runtime.Env)+len(service.Env))
	for key, value := range p.Runtime.Env {
		merged[key] = value
	}
	for key, value := range service.Env {
		merged[key] = value
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func (p *Project) RestartPolicy() string {
	if p.Runtime.Restart == "" {
		return "unless-stopped"
	}
	return p.Runtime.Restart
}
