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
	"time"

	"github.com/mustafanass/uruflow/internal/agent/runner"
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
	system, err := d.metrics.Collect()
	if err != nil {
		logger.Error("[AGENT] metrics collection failed: %v", err)
		return
	}

	containers, available := d.collectContainers(ctx)
	payload := ufp.Metrics{
		Timestamp: time.Now().Unix(),
		System: ufp.SystemMetrics{
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
			ID:           shortID(container.ID),
			Name:         container.Name,
			Project:      container.Project,
			Service:      container.Service,
			Image:        container.Image,
			State:        container.State,
			Health:       container.Health,
			RestartCount: container.RestartCount,
			StartedAt:    container.StartedAt,
		}

		if container.State == "running" {
			statsCtx, stop := context.WithTimeout(ctx, statsTimeout)
			if stats, err := d.docker.Stats(statsCtx, container.ID); err == nil {
				status.CPUPercent = stats.CPUPercent
				status.MemoryUsage = stats.MemoryUsage
				status.MemoryLimit = stats.MemoryLimit
				status.NetworkRx = stats.NetworkRx
				status.NetworkTx = stats.NetworkTx
			}
			stop()
		}

		reported = append(reported, status)
	}

	return reported, true
}

func shortID(id string) string {
	if len(id) <= shortIDSize {
		return id
	}
	return id[:shortIDSize]
}
