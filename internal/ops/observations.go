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

package ops

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/observe"
	"github.com/mustafanass/uruflow/internal/version"
)

const ObservationInterval = 2 * time.Second

func (e *Engine) Snapshot(ctx context.Context) (observe.Snapshot, error) {
	s := observe.Snapshot{At: time.Now(), Version: version.Current, RegistryAddress: e.server.Config().Registry.Address()}
	s.ServerName, _ = os.Hostname()
	store := e.server.Store()
	var err error
	if s.Agents, err = store.ListAgents(); err != nil {
		return s, err
	}
	if s.Projects, err = store.ListProjects(); err != nil {
		return s, err
	}
	if s.Containers, err = store.ListContainers(); err != nil {
		return s, err
	}
	e.releaseMu.Lock()
	revision, releaseErr := store.ReleaseRevision()
	if releaseErr == nil && (e.releaseCache == nil || e.releaseRevision != revision) {
		e.releaseCache, releaseErr = store.SnapshotReleases()
		if releaseErr == nil {
			e.releaseRevision = revision
		}
	}
	s.Releases = append([]models.Release(nil), e.releaseCache...)
	e.releaseMu.Unlock()
	if releaseErr != nil {
		return s, releaseErr
	}
	if s.Alerts, err = store.ListActiveAlerts(); err != nil {
		return s, err
	}
	for i := range s.Projects {
		s.Projects[i] = observe.RedactProject(s.Projects[i])
	}
	for _, p := range e.server.ProjectProblems() {
		s.Problems = append(s.Problems, p.Path+": "+fmt.Sprint(p.Reason))
	}
	if feed := e.server.Activity(); feed != nil {
		entries, _ := feed.Read(0)
		if len(entries) > 120 {
			entries = entries[len(entries)-120:]
		}
		s.Activity = entries
	}
	e.observationMu.Lock()
	defer e.observationMu.Unlock()
	if e.hostCache == nil || time.Since(e.hostCache.CollectedAt) >= ObservationInterval {
		sys, err := e.hostCollector.Collect()
		if err != nil {
			return s, err
		}
		e.hostCache = &models.Metrics{CollectedAt: time.Now(), Available: sys.Available, Errors: sys.Errors, CPUCores: sys.CPUCores, HostUptime: sys.HostUptime,
			CPUPercent: sys.CPUPercent, MemoryPercent: sys.MemoryPercent, MemoryUsed: sys.MemoryUsed, MemoryTotal: sys.MemoryTotal,
			DiskPercent: sys.DiskPercent, DiskUsed: sys.DiskUsed, DiskTotal: sys.DiskTotal, LoadAvg: sys.LoadAvg, Uptime: sys.Uptime}
	}
	s.Host = e.hostCache
	if e.server.Registry().Maintaining() {
		e.registryState = "maintenance"
		e.registryChecked = time.Now()
	} else if e.registryState == "maintenance" || time.Since(e.registryChecked) > 15*time.Second {
		healthCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		e.registryState = "healthy"
		if e.server.Registry().Health(healthCtx) != nil {
			e.registryState = "unavailable"
		}
		cancel()
		e.registryChecked = time.Now()
	}
	s.Registry = e.registryState
	s.Graphs = make(map[string]observe.ProjectGraphs, len(s.Projects))
	s.ProjectStates = make(map[string]string, len(s.Projects))
	for _, p := range s.Projects {
		s.Graphs[p.Name] = s.ProjectGraphs(p)
		s.ProjectStates[p.Name] = s.ProjectState(p)
	}
	s.Fleet = s.FleetGraph()
	for i := range s.Releases {
		s.Releases[i].Spec = models.Project{}
	}
	return s, nil
}
