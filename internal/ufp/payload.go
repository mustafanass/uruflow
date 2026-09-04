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

package ufp

import "time"

type Hello struct {
	AgentID  string `json:"agent_id"`
	Hostname string `json:"hostname"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Roles    []Role `json:"roles"`
}

type Challenge struct {
	Nonce []byte `json:"nonce"`
}

type Proof struct {
	Proof []byte `json:"proof"`
}

type Welcome struct {
	AgentID       string `json:"agent_id"`
	Name          string `json:"name"`
	ServerVersion string `json:"server_version"`
}

type Reject struct {
	Reason string `json:"reason"`
}

type Goodbye struct {
	Reason string `json:"reason"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

type Accepted struct {
	JobID string `json:"job_id,omitempty"`
}

type RegistryConfig struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	CACert   string `json:"ca_cert"`
}

type BuildRequest struct {
	JobID   string        `json:"job_id"`
	Project string        `json:"project"`
	Timeout time.Duration `json:"timeout,omitempty"`
	Tags    []string      `json:"tags"`
	Targets []BuildTarget `json:"targets"`
}

type BuildTarget struct {
	Service    string            `json:"service"`
	Image      string            `json:"image"`
	Dockerfile string            `json:"dockerfile"`
	Context    string            `json:"context"`
	BuildArgs  map[string]string `json:"build_args,omitempty"`
	GitURL     string            `json:"git_url,omitempty"`
	Branch     string            `json:"branch,omitempty"`
	Commit     string            `json:"commit,omitempty"`
}

type ReleaseRequest struct {
	JobID    string                     `json:"job_id"`
	Project  string                     `json:"project"`
	Timeout  time.Duration              `json:"timeout,omitempty"`
	Services []ServiceSpec              `json:"services"`
	Networks map[string]NetworkResource `json:"networks,omitempty"`
	Volumes  map[string]VolumeResource  `json:"volumes,omitempty"`
}

type ServiceSpec struct {
	Name        string              `json:"name"`
	Image       string              `json:"image"`
	Ports       []PortBinding       `json:"ports,omitempty"`
	Env         map[string]string   `json:"env,omitempty"`
	Volumes     []VolumeBinding     `json:"volumes,omitempty"`
	Network     string              `json:"network,omitempty"`
	Networks    []NetworkAttachment `json:"networks,omitempty"`
	Restart     string              `json:"restart,omitempty"`
	Command     string              `json:"command,omitempty"`
	CommandExec []string            `json:"command_exec,omitempty"`
	Entrypoint  []string            `json:"entrypoint,omitempty"`
	Mode        string              `json:"mode,omitempty"`
	DependsOn   []Dependency        `json:"depends_on,omitempty"`
	Resources   ResourceLimits      `json:"resources,omitempty"`
	Security    SecuritySpec        `json:"security,omitempty"`
	Logging     LogConfig           `json:"logging,omitempty"`
	JobTimeout  time.Duration       `json:"job_timeout,omitempty"`
	Healthcheck *HealthcheckSpec    `json:"healthcheck,omitempty"`
	Labels      map[string]string   `json:"labels,omitempty"`
}

type HealthcheckSpec struct {
	Type        string        `json:"type"`
	Scheme      string        `json:"scheme,omitempty"`
	Path        string        `json:"path,omitempty"`
	Port        int           `json:"port,omitempty"`
	Interval    time.Duration `json:"interval,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	Retries     int           `json:"retries,omitempty"`
	StableFor   time.Duration `json:"stable_for,omitempty"`
	Command     []string      `json:"command,omitempty"`
	StartPeriod time.Duration `json:"start_period,omitempty"`
}

type PortBinding struct {
	HostIP    string `json:"host_ip,omitempty"`
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol,omitempty"`
}

type NetworkAttachment struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
}

type Dependency struct {
	Service   string `json:"service"`
	Condition string `json:"condition"`
}

type ResourceLimits struct {
	MemoryBytes int64   `json:"memory_bytes,omitempty"`
	CPUs        float64 `json:"cpus,omitempty"`
	PIDs        int64   `json:"pids,omitempty"`
}

type SecuritySpec struct {
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

type VolumeBinding struct {
	Type           string `json:"type,omitempty"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	ReadOnly       bool   `json:"read_only,omitempty"`
	CreateHostPath bool   `json:"create_host_path,omitempty"`
}

type ProjectRef struct {
	JobID   string `json:"job_id,omitempty"`
	Project string `json:"project"`
}

type JobLog struct {
	JobID     string `json:"job_id"`
	Stage     string `json:"stage"`
	Stream    string `json:"stream"`
	Line      string `json:"line"`
	Timestamp int64  `json:"timestamp"`
}

type JobStatus struct {
	JobID    string            `json:"job_id"`
	Stage    string            `json:"stage"`
	Status   string            `json:"status"`
	Message  string            `json:"message,omitempty"`
	Image    string            `json:"image,omitempty"`
	Images   map[string]string `json:"images,omitempty"`
	Digest   string            `json:"digest,omitempty"`
	Commit   string            `json:"commit,omitempty"`
	Commits  map[string]string `json:"commits,omitempty"`
	Duration int64             `json:"duration"`
}

type Metrics struct {
	Timestamp           int64             `json:"timestamp"`
	System              SystemMetrics     `json:"system"`
	ContainersAvailable bool              `json:"containers_available"`
	Containers          []ContainerStatus `json:"containers,omitempty"`
}

type SystemMetrics struct {
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

type ContainerStatus struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Project      string  `json:"project"`
	Service      string  `json:"service,omitempty"`
	Image        string  `json:"image"`
	State        string  `json:"state"`
	Health       string  `json:"health"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemoryUsage  uint64  `json:"memory_usage"`
	MemoryLimit  uint64  `json:"memory_limit"`
	NetworkRx    uint64  `json:"network_rx"`
	NetworkTx    uint64  `json:"network_tx"`
	RestartCount int     `json:"restart_count"`
	StartedAt    int64   `json:"started_at"`
}

type LogsFollow struct {
	ContainerID string `json:"container_id"`
	Tail        int    `json:"tail"`
}

type LogsStop struct {
	ContainerID string `json:"container_id"`
}

type ContainerLog struct {
	ContainerID string `json:"container_id"`
	Stream      string `json:"stream"`
	Line        string `json:"line"`
	Timestamp   int64  `json:"timestamp"`
}
