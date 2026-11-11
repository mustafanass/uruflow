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

package daemon

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/mustafanass/uruflow/internal/agent/runner"
	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const (
	listTimeout  = 10 * time.Second
	statsTimeout = 5 * time.Second
	shortIDSize  = 12
)

func (d *Daemon) reportMetrics(ctx context.Context) {
	interval := time.Duration(d.cfg.Server.MetricsSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	d.publishMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.publishMetrics(ctx)
		}
	}
}

func (d *Daemon) publishMetrics(ctx context.Context) {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()
	sampledAt := time.Now()
	system, err := d.metrics.Collect()
	if err != nil {
		logger.Error("[AGENT] metrics collection failed: %v", err)
		return
	}

	containers, available := d.collectContainers(ctx)
	payload := ufp.Metrics{
		Timestamp: sampledAt.Unix(),
		System: ufp.SystemMetrics{
			MetricsSec: d.cfg.Server.MetricsSec,
			Available:  system.Available, Errors: system.Errors, CPUCores: system.CPUCores, HostUptime: system.HostUptime,
			CPUPercent:    system.CPUPercent,
			MemoryPercent: system.MemoryPercent,
			MemoryUsed:    system.MemoryUsed,
			MemoryTotal:   system.MemoryTotal,
			DiskPercent:   system.DiskPercent,
			DiskUsed:      system.DiskUsed,
			DiskTotal:     system.DiskTotal,
			LoadAvg:       system.LoadAvg,
			Uptime:        system.Uptime,
		},
		ContainersAvailable: available,
		Containers:          containers,
	}

	d.send(ufp.TopicMetrics, payload)
}

func (d *Daemon) collectContainers(ctx context.Context) ([]ufp.ContainerStatus, bool) {
	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	containers, err := d.docker.ListContainers(listCtx, true)
	if err != nil {
		logger.Warn("[AGENT] container listing failed: %v", err)
		return nil, false
	}

	reported := make([]ufp.ContainerStatus, 0, len(containers))
	for _, container := range containers {
		if strings.HasSuffix(container.Name, runner.PreviousSuffix) {
			continue
		}

		status := ufp.ContainerStatus{
			ID:           container.ID,
			Details:      container.Details,
			Name:         container.Name,
			Project:      container.Project,
			Service:      container.Service,
			Image:        container.Image,
			State:        container.State,
			Health:       container.Health,
			RestartCount: container.RestartCount,
			StartedAt:    container.StartedAt,
		}

		reported = append(reported, status)
	}

	counters := d.collectStats(ctx, reported)
	previous := d.containerCPU
	d.containerCPU = make(map[string]docker.CPUCounters, len(reported))
	for index := range reported {
		if counters[index].System == 0 {
			continue
		}
		status := &reported[index]
		d.containerCPU[status.ID] = counters[index]
		percent, ok := counters[index].Percent(previous[status.ID])
		if !ok {
			continue
		}
		status.CPUPercent = percent
		status.StatsAvailable = true
	}

	return reported, true
}

func (d *Daemon) collectStats(ctx context.Context, reported []ufp.ContainerStatus) []docker.CPUCounters {
	counters := make([]docker.CPUCounters, len(reported))
	statsCtx, stopAll := context.WithTimeout(ctx, statsTimeout)
	defer stopAll()
	var wg sync.WaitGroup
	slots := make(chan struct{}, 4)
	for index := range reported {
		if reported[index].State != "running" {
			continue
		}
		select {
		case slots <- struct{}{}:
		case <-statsCtx.Done():
			wg.Wait()
			return counters
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			defer func() { <-slots }()
			stats, err := d.docker.Stats(statsCtx, reported[index].ID)
			if err != nil {
				return
			}
			status := &reported[index]
			status.MemoryUsage, status.MemoryLimit = stats.MemoryUsage, stats.MemoryLimit
			status.NetworkRx, status.NetworkTx = stats.NetworkRx, stats.NetworkTx
			status.PIDs, status.BlockRead, status.BlockWrite = stats.PIDs, stats.BlockRead, stats.BlockWrite
			counters[index] = stats.CPU
		}(index)
	}
	wg.Wait()

	return counters
}

func shortID(id string) string {
	if len(id) <= shortIDSize {
		return id
	}
	return id[:shortIDSize]
}
