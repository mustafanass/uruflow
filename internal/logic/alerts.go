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

package logic

import (
	"fmt"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/pkg/helper"
)

const (
	KindCPU           = "high_cpu"
	KindMemory        = "high_memory"
	KindDisk          = "high_disk"
	KindContainerDown = "container_down"
	KindAgentOffline  = "agent_offline"
)

const (
	CPUWarning     = 80.0
	CPUCritical    = 90.0
	MemoryWarning  = 90.0
	MemoryCritical = 95.0
	DiskWarning    = 85.0
	DiskCritical   = 95.0
)

type threshold struct {
	label    string
	warning  float64
	critical float64
}

var resourceThresholds = map[string]threshold{
	KindCPU:    {label: "CPU", warning: CPUWarning, critical: CPUCritical},
	KindMemory: {label: "Memory", warning: MemoryWarning, critical: MemoryCritical},
	KindDisk:   {label: "Disk", warning: DiskWarning, critical: DiskCritical},
}

func CheckResource(agentID, agentName, kind string, percent float64) *models.Alert {
	limits, known := resourceThresholds[kind]
	if !known {
		return nil
	}

	switch {
	case percent >= limits.critical:
		return newAlert(agentID, agentName, kind,
			fmt.Sprintf("%s usage above %.0f%%", limits.label, limits.critical), models.SeverityCritical)
	case percent >= limits.warning:
		return newAlert(agentID, agentName, kind,
			fmt.Sprintf("%s usage above %.0f%%", limits.label, limits.warning), models.SeverityWarning)
	}
	return nil
}

func CheckContainerDown(agentID, agentName, container string) *models.Alert {
	return newAlert(agentID, agentName, KindContainerDown,
		fmt.Sprintf("Container %s is not running", container), models.SeverityCritical)
}

func CheckAgentOffline(agentID, agentName string) *models.Alert {
	return newAlert(agentID, agentName, KindAgentOffline,
		fmt.Sprintf("Agent %s went offline", agentName), models.SeverityCritical)
}

func newAlert(agentID, agentName, kind, message string, severity models.Severity) *models.Alert {
	return &models.Alert{
		ID:        helper.GenerateID(),
		AgentID:   agentID,
		AgentName: agentName,
		Type:      kind,
		Message:   message,
		Severity:  severity,
		CreatedAt: time.Now(),
	}
}
