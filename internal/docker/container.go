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
	"github.com/mustafanass/uruflow/internal/ufp"
	"net"
	"sort"
	"strconv"
	"sync"
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
)

type State struct {
	ID       string
	Image    string
	SpecHash string
	Status   string
	Health   string
	ExitCode int
	Restarts int
}

type Container struct {
	Details      *ufp.ContainerDetails
	PIDs         uint64
	BlockRead    uint64
	BlockWrite   uint64
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
	CPU          CPUCounters
	MemoryUsage  uint64
	MemoryLimit  uint64
	NetworkRx    uint64
	NetworkTx    uint64
}

type CPUCounters struct {
	Container uint64
	System    uint64
	Cores     int
}

type Spec struct {
	Name        string
	Image       string
	Entrypoint  []string
	Command     []string
	Env         map[string]string
	Ports       []PortBinding
	Mounts      []Mount
	Labels      map[string]string
	Network     string
	Networks    []NetworkAttachment
	Restart     string
	Resources   ResourceLimits
	Security    Security
	Logging     LogConfig
	Healthcheck *Healthcheck
}

type PortBinding struct {
	HostIP    string
	Host      int
	Container int
	Protocol  string
}

type NetworkAttachment struct {
	Name    string
	Aliases []string
}

type ResourceLimits struct {
	MemoryBytes int64
	CPUs        float64
	PIDs        int64
}

type Security struct {
	NoNewPrivileges bool
	ReadOnlyRootFS  bool
	User            string
	CapAdd          []string
	CapDrop         []string
}

type LogConfig struct {
	Driver  string
	Options map[string]string
}

type Healthcheck struct {
	Test        []string
	Interval    time.Duration
	Timeout     time.Duration
	Retries     int
	StartPeriod time.Duration
}

type Mount struct {
	Type           string
	Source         string
	Target         string
	ReadOnly       bool
	CreateHostPath bool
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
	ID      string `json:"Id"`
	Image   string `json:"Image"`
	Created string `json:"Created"`
	Mounts  []struct {
		Type        string
		Source      string
		Destination string
		RW          bool
	} `json:"Mounts"`
	State struct {
		Status     string `json:"Status"`
		ExitCode   int    `json:"ExitCode"`
		OOMKilled  bool   `json:"OOMKilled"`
		Error      string `json:"Error"`
		FinishedAt string `json:"FinishedAt"`
		StartedAt  string `json:"StartedAt"`
		Health     *struct {
			Status string `json:"Status"`
			Log    []struct {
				End      string
				ExitCode int
				Output   string
			} `json:"Log"`
		} `json:"Health"`
	} `json:"State"`
	RestartCount int `json:"RestartCount"`
	Config       struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	NetworkSettings struct {
		IPAddress string `json:"IPAddress"`
		Ports     map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
		Networks map[string]struct {
			IPAddress string   `json:"IPAddress"`
			Gateway   string   `json:"Gateway"`
			Aliases   []string `json:"Aliases"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
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

		containers = append(containers, container)
	}

	var wg sync.WaitGroup
	slots := make(chan struct{}, 4)
	for index := range containers {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return containers, nil
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			defer func() { <-slots }()
			details, err := c.Inspect(ctx, containers[index].ID)
			if err != nil {
				return
			}
			container := &containers[index]
			container.RestartCount = details.RestartCount
			if details.State.Health != nil && details.State.Health.Status != "" {
				container.Health = details.State.Health.Status
			}
			if started, err := time.Parse(time.RFC3339Nano, details.State.StartedAt); err == nil {
				container.StartedAt = started.Unix()
			}
			container.Details = runtimeDetails(details)
		}(index)
	}
	wg.Wait()

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

func (c *Client) Endpoint(ctx context.Context, id string, port int) (string, error) {
	details, err := c.Inspect(ctx, id)
	if err != nil {
		return "", err
	}

	bindings := details.NetworkSettings.Ports[fmt.Sprintf("%d/tcp", port)]
	for _, binding := range bindings {
		if binding.HostPort == "" {
			continue
		}
		host := binding.HostIP
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		return net.JoinHostPort(host, binding.HostPort), nil
	}

	addresses := make([]string, 0, len(details.NetworkSettings.Networks)+1)
	if details.NetworkSettings.IPAddress != "" {
		addresses = append(addresses, details.NetworkSettings.IPAddress)
	}
	for _, network := range details.NetworkSettings.Networks {
		if network.IPAddress != "" {
			addresses = append(addresses, network.IPAddress)
		}
	}
	if len(addresses) == 0 {
		return "", fmt.Errorf("container has no reachable address for port %d", port)
	}
	sort.Strings(addresses)
	return net.JoinHostPort(addresses[0], strconv.Itoa(port)), nil
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
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
		PIDsStats struct {
			Current uint64 `json:"current"`
		} `json:"pids_stats"`
		BlockStats struct {
			Entries []struct {
				Op    string
				Value uint64
			} `json:"io_service_bytes_recursive"`
		} `json:"blkio_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}

	if err := c.get(ctx, "/containers/"+id+"/stats?stream=false&one-shot=true", &stats); err != nil {
		return nil, err
	}

	container := &Container{
		MemoryUsage: stats.MemoryStats.Usage,
		PIDs:        stats.PIDsStats.Current,
		MemoryLimit: stats.MemoryStats.Limit,
		CPU: CPUCounters{
			Container: stats.CPUStats.CPUUsage.TotalUsage,
			System:    stats.CPUStats.SystemUsage,
			Cores:     stats.CPUStats.OnlineCPUs,
		},
	}

	for _, network := range stats.Networks {
		container.NetworkRx += network.RxBytes
		container.NetworkTx += network.TxBytes
	}

	for _, entry := range stats.BlockStats.Entries {
		switch entry.Op {
		case "Read", "read":
			container.BlockRead += entry.Value
		case "Write", "write":
			container.BlockWrite += entry.Value
		}
	}
	return container, nil
}

func (c CPUCounters) Percent(previous CPUCounters) (float64, bool) {
	if previous.System == 0 || c.System <= previous.System || c.Container < previous.Container {
		return 0, false
	}
	cores := c.Cores
	if cores == 0 {
		cores = 1
	}
	return float64(c.Container-previous.Container) / float64(c.System-previous.System) * float64(cores) * 100, true
}

func runtimeDetails(details *inspectResult) *ufp.ContainerDetails {
	result := &ufp.ContainerDetails{ImageID: details.Image, Created: details.Created,
		Release: details.Config.Labels[LabelRelease], Role: details.Config.Labels[LabelRole],
		ExitCode: details.State.ExitCode, OOMKilled: details.State.OOMKilled,
		Error: details.State.Error, FinishedAt: details.State.FinishedAt}
	names := make([]string, 0, len(details.NetworkSettings.Networks))
	for name := range details.NetworkSettings.Networks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		network := details.NetworkSettings.Networks[name]
		result.Networks = append(result.Networks,
			ufp.ContainerNetwork{Name: name, Address: network.IPAddress, Gateway: network.Gateway, Aliases: network.Aliases})
	}
	for port, bindings := range details.NetworkSettings.Ports {
		for _, binding := range bindings {
			result.Ports = append(result.Ports, net.JoinHostPort(binding.HostIP, binding.HostPort)+" → "+port)
		}
		if len(bindings) == 0 {
			result.Ports = append(result.Ports, port+" (internal)")
		}
	}
	sort.Strings(result.Ports)
	for _, mount := range details.Mounts {
		result.Mounts = append(result.Mounts, ufp.ContainerMount{Type: mount.Type, Source: mount.Source, Target: mount.Destination, ReadOnly: !mount.RW})
	}
	if details.State.Health != nil {
		for _, probe := range details.State.Health.Log {
			output := probe.Output
			if len(output) > 1024 {
				output = output[:1024] + "…"
			}
			result.Probes = append(result.Probes, ufp.HealthProbe{End: probe.End, ExitCode: probe.ExitCode, Output: output})
		}
	}
	return result
}
