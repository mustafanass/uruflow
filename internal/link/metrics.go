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

package link

import (
	"time"

	"github.com/urustack/uruflow/internal/logic"
	"github.com/urustack/uruflow/internal/models"
	"github.com/urustack/uruflow/internal/ufp"
)

func (s *Server) applyMetrics(identity *ufp.Identity, metrics ufp.Metrics) {
	system := &models.Metrics{
		CPUPercent:    metrics.System.CPUPercent,
		MemoryPercent: metrics.System.MemoryPercent,
		MemoryUsed:    metrics.System.MemoryUsed,
		MemoryTotal:   metrics.System.MemoryTotal,
		DiskPercent:   metrics.System.DiskPercent,
		DiskUsed:      metrics.System.DiskUsed,
		DiskTotal:     metrics.System.DiskTotal,
		LoadAvg:       metrics.System.LoadAvg,
		Uptime:        metrics.System.Uptime,
	}
	s.store.SetAgentMetrics(identity.AgentID, system)

	containers := make([]models.Container, 0, len(metrics.Containers))
	for _, reported := range metrics.Containers {
		containers = append(containers, models.Container{
			ID:           reported.ID,
			AgentID:      identity.AgentID,
			Name:         reported.Name,
			Project:      reported.Project,
			Service:      reported.Service,
			Image:        reported.Image,
			State:        reported.State,
			Health:       reported.Health,
			CPUPercent:   reported.CPUPercent,
			MemoryUsage:  reported.MemoryUsage,
			MemoryLimit:  reported.MemoryLimit,
			NetworkRx:    reported.NetworkRx,
			NetworkTx:    reported.NetworkTx,
			RestartCount: reported.RestartCount,
			StartedAt:    time.Unix(reported.StartedAt, 0),
		})
	}
	s.store.ReplaceContainers(identity.AgentID, containers)

	s.evaluateAlerts(identity, metrics, containers)
}

func (s *Server) evaluateAlerts(identity *ufp.Identity, metrics ufp.Metrics, containers []models.Container) {
	active, err := s.store.ListActiveAlerts()
	if err != nil {
		return
	}

	open := make(map[string]models.Alert, len(active))
	for _, alert := range active {
		if alert.AgentID == identity.AgentID {
			open[alert.Message] = alert
		}
	}

	raise := func(alert *models.Alert) {
		if alert == nil {
			return
		}
		if _, exists := open[alert.Message]; exists {
			delete(open, alert.Message)
			return
		}
		s.store.CreateAlert(alert)
	}

	raise(logic.CheckResource(identity.AgentID, identity.Name, logic.KindCPU, metrics.System.CPUPercent))
	raise(logic.CheckResource(identity.AgentID, identity.Name, logic.KindMemory, metrics.System.MemoryPercent))
	raise(logic.CheckResource(identity.AgentID, identity.Name, logic.KindDisk, metrics.System.DiskPercent))

	for _, container := range containers {
		if container.State == "running" {
			continue
		}
		raise(logic.CheckContainerDown(identity.AgentID, identity.Name, container.Name))
	}

	for _, stale := range open {
		if stale.Type != logic.KindAgentOffline {
			s.store.ResolveAlert(stale.ID)
		}
	}
}

func (s *Server) raiseAlert(alert *models.Alert) {
	if alert == nil {
		return
	}

	active, err := s.store.ListActiveAlerts()
	if err == nil {
		for _, existing := range active {
			if existing.AgentID == alert.AgentID && existing.Message == alert.Message {
				return
			}
		}
	}
	s.store.CreateAlert(alert)
}
