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

package docker

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	StateRunning    = "running"
	StateExited     = "exited"
	StateDead       = "dead"
	StateRestarting = "restarting"

	HealthNone      = "none"
	HealthStarting  = "starting"
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"

	pollInterval = 500 * time.Millisecond
)

type State struct {
	Status   string
	Health   string
	ExitCode int
	Restarts int
}

type Container struct {
	ID           string
	Name         string
	Project      string
	Service      string
	Image        string
	State        string
	Status       string
	Health       string
	Managed      bool
	RestartCount int
	StartedAt    int64
	CPUPercent   float64
	MemoryUsage  uint64
	MemoryLimit  uint64
	NetworkRx    uint64
	NetworkTx    uint64
}

type Spec struct {
	Name    string
	Image   string
	Command []string
	Env     map[string]string
	Ports   []PortBinding
	Mounts  []Mount
	Labels  map[string]string
	Network string
	Restart string
}

type PortBinding struct {
	Host      int
	Container int
	Protocol  string
}

type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type listEntry struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Labels map[string]string `json:"Labels"`
}

type inspectResult struct {
	State struct {
		Status    string `json:"Status"`
		ExitCode  int    `json:"ExitCode"`
		StartedAt string `json:"StartedAt"`
		Health    *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	RestartCount int `json:"RestartCount"`
	Config       struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

func (c *Client) ListContainers(ctx context.Context, managedOnly bool) ([]Container, error) {
	var entries []listEntry
	if err := c.get(ctx, "/containers/json?all=true", &entries); err != nil {
		return nil, err
	}

	containers := make([]Container, 0, len(entries))
	for _, entry := range entries {
		managed := IsManaged(entry.Labels)
		if managedOnly && !managed {
			continue
		}

		container := Container{
			ID:      entry.ID,
			Name:    containerName(entry.Names),
			Project: ProjectOf(entry.Labels),
			Service: ServiceOf(entry.Labels),
			Image:   entry.Image,
			State:   entry.State,
			Status:  entry.Status,
			Health:  "none",
			Managed: managed,
		}

		if details, err := c.Inspect(ctx, entry.ID); err == nil {
			container.RestartCount = details.RestartCount
			if details.State.Health != nil && details.State.Health.Status != "" {
				container.Health = details.State.Health.Status
			}
			if started, err := time.Parse(time.RFC3339Nano, details.State.StartedAt); err == nil {
				container.StartedAt = started.Unix()
			}
		}

		containers = append(containers, container)
	}

	sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
	return containers, nil
}

func (c *Client) Inspect(ctx context.Context, id string) (*inspectResult, error) {
	var result inspectResult
	if err := c.get(ctx, "/containers/"+id+"/json", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Exists(ctx context.Context, name string) bool {
	exists, _ := c.ContainerExists(ctx, name)
	return exists
}

func (c *Client) ContainerExists(ctx context.Context, name string) (bool, error) {
	_, err := c.Inspect(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) ContainerOwnership(ctx context.Context, name, project string) (bool, bool, error) {
	details, err := c.Inspect(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	labels := details.Config.Labels
	return true, IsManaged(labels) && ProjectOf(labels) == project, nil
}

func (c *Client) Stats(ctx context.Context, id string) (*Container, error) {
	var stats struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs  int    `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}

	if err := c.get(ctx, "/containers/"+id+"/stats?stream=false", &stats); err != nil {
		return nil, err
	}

	container := &Container{
		MemoryUsage: stats.MemoryStats.Usage,
		MemoryLimit: stats.MemoryStats.Limit,
	}

	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	if cpuDelta > 0 && systemDelta > 0 {
		cores := stats.CPUStats.OnlineCPUs
		if cores == 0 {
			cores = 1
		}
		container.CPUPercent = cpuDelta / systemDelta * float64(cores) * 100
	}

	for _, network := range stats.Networks {
		container.NetworkRx += network.RxBytes
		container.NetworkTx += network.TxBytes
	}

	return container, nil
}

func (c *Client) Create(ctx context.Context, spec Spec) (string, error) {
	body := map[string]any{
		"Image":      spec.Image,
		"Labels":     spec.Labels,
		"HostConfig": hostConfig(spec),
	}

	if len(spec.Command) > 0 {
		body["Cmd"] = spec.Command
	}
	if len(spec.Env) > 0 {
		body["Env"] = environment(spec.Env)
	}
	if exposed := exposedPorts(spec.Ports); len(exposed) > 0 {
		body["ExposedPorts"] = exposed
	}

	var created struct {
		ID string `json:"Id"`
	}
	path := "/containers/create?name=" + url.QueryEscape(spec.Name)
	if err := c.post(ctx, path, body, &created); err != nil {
		return "", err
	}
	return created.ID, nil
}

func (c *Client) Start(ctx context.Context, id string) error {
	return c.post(ctx, "/containers/"+id+"/start", nil, nil)
}

func (c *Client) Stop(ctx context.Context, id string, timeout time.Duration) error {
	return c.post(ctx, fmt.Sprintf("/containers/%s/stop?t=%d", id, int(timeout.Seconds())), nil, nil)
}

func (c *Client) Remove(ctx context.Context, id string, force bool) error {
	return c.delete(ctx, fmt.Sprintf("/containers/%s?force=%t&v=true", id, force))
}

func (c *Client) Rename(ctx context.Context, id, name string) error {
	return c.post(ctx, "/containers/"+id+"/rename?name="+url.QueryEscape(name), nil, nil)
}

func (c *Client) State(ctx context.Context, id string) (*State, error) {
	details, err := c.Inspect(ctx, id)
	if err != nil {
		return nil, err
	}

	state := &State{
		Status:   details.State.Status,
		Health:   HealthNone,
		ExitCode: details.State.ExitCode,
		Restarts: details.RestartCount,
	}
	if details.State.Health != nil && details.State.Health.Status != "" {
		state.Health = details.State.Health.Status
	}
	return state, nil
}

func (s *State) Failed() error {
	switch {
	case s.Status == StateExited || s.Status == StateDead:
		return fmt.Errorf("container exited with code %d", s.ExitCode)
	case s.Health == HealthUnhealthy:
		return errors.New("container reported itself unhealthy")
	default:
		return nil
	}
}

func (s *State) Ready() bool {
	if s.Status != StateRunning {
		return false
	}
	return s.Health == HealthHealthy || s.Health == HealthNone
}

func (c *Client) WaitReady(ctx context.Context, id string, settle, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var readySince time.Time
	restarts := -1

	for time.Now().Before(deadline) {
		state, err := c.State(ctx, id)
		if err != nil {
			return err
		}
		if err := state.Failed(); err != nil {
			return err
		}

		if restarts < 0 {
			restarts = state.Restarts
		}
		if state.Restarts > restarts {
			return fmt.Errorf("container restarted %d time(s) while coming up", state.Restarts-restarts)
		}

		switch {
		case !state.Ready():
			readySince = time.Time{}
		case state.Health == HealthHealthy:
			return nil
		case readySince.IsZero():
			readySince = time.Now()
		case time.Since(readySince) >= settle:
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return fmt.Errorf("container did not become ready within %s", timeout)
}

func (c *Client) Run(ctx context.Context, spec Spec) (string, error) {
	id, err := c.Create(ctx, spec)
	if err != nil {
		return "", err
	}
	if err := c.Start(ctx, id); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), requestLimit)
		cleanupErr := c.Remove(cleanupCtx, id, true)
		cancel()
		if cleanupErr != nil {
			return "", errors.Join(err, fmt.Errorf("remove failed container: %w", cleanupErr))
		}
		return "", err
	}
	return id, nil
}

func hostConfig(spec Spec) map[string]any {
	config := map[string]any{
		"RestartPolicy": map[string]any{"Name": restartPolicy(spec.Restart)},
	}

	if bindings := portBindings(spec.Ports); len(bindings) > 0 {
		config["PortBindings"] = bindings
	}
	if binds := mountBinds(spec.Mounts); len(binds) > 0 {
		config["Binds"] = binds
	}
	if spec.Network != "" {
		config["NetworkMode"] = spec.Network
	}

	return config
}

func restartPolicy(policy string) string {
	if policy == "" {
		return "unless-stopped"
	}
	return policy
}

func environment(env map[string]string) []string {
	values := make([]string, 0, len(env))
	for key, value := range env {
		values = append(values, key+"="+value)
	}
	sort.Strings(values)
	return values
}

func portKey(port PortBinding) string {
	protocol := port.Protocol
	if protocol == "" {
		protocol = "tcp"
	}
	return fmt.Sprintf("%d/%s", port.Container, protocol)
}

func exposedPorts(ports []PortBinding) map[string]any {
	exposed := make(map[string]any, len(ports))
	for _, port := range ports {
		exposed[portKey(port)] = struct{}{}
	}
	return exposed
}

func portBindings(ports []PortBinding) map[string]any {
	bindings := make(map[string]any, len(ports))
	for _, port := range ports {
		bindings[portKey(port)] = []map[string]string{
			{"HostPort": fmt.Sprint(port.Host)},
		}
	}
	return bindings
}

func mountBinds(mounts []Mount) []string {
	binds := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		bind := mount.Source + ":" + mount.Target
		if mount.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}
	return binds
}

func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}
