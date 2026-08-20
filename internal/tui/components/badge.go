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

package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/urustack/uruflow/internal/models"
	"github.com/urustack/uruflow/internal/tui/theme"
)

func AgentBadge(status models.AgentStatus) Cell {
	if status == models.AgentOnline {
		return Styled(theme.IconOnline+" online", theme.Good)
	}
	return Styled(theme.IconOffline+" offline", theme.Ghost)
}

func StatusBadge(status models.Status) Cell {
	icon, style := statusLook(status)
	return Styled(icon+" "+string(status), style)
}

func StatusStyle(status models.Status) lipgloss.Style {
	_, style := statusLook(status)
	return style
}

func StatusIcon(status models.Status) string {
	icon, _ := statusLook(status)
	return icon
}

func statusLook(status models.Status) (string, lipgloss.Style) {
	switch status {
	case models.StatusSucceeded:
		return theme.IconSuccess, theme.Good
	case models.StatusFailed:
		return theme.IconFailure, theme.Bad
	case models.StatusBuilding:
		return theme.IconRunning, theme.Mark
	case models.StatusReleasing:
		return theme.IconRunning, theme.Lead
	case models.StatusSkipped:
		return theme.IconSkipped, theme.Ghost
	default:
		return theme.IconPending, theme.Faint
	}
}

func ContainerBadge(state string) Cell {
	switch state {
	case "running":
		return Styled(theme.IconOnline+" running", theme.Good)
	case "restarting", "created", "paused":
		return Styled(theme.IconRunning+" "+state, theme.Warn)
	default:
		return Styled(theme.IconOffline+" "+state, theme.Bad)
	}
}

func SeverityBadge(severity models.Severity) Cell {
	if severity == models.SeverityCritical {
		return Styled(theme.IconFailure+" critical", theme.Bad)
	}
	return Styled(theme.IconWarning+" warning", theme.Warn)
}

func Roles(roles []models.Role) Cell {
	label := ""
	for _, role := range roles {
		if label != "" {
			label += " "
		}
		switch role {
		case models.RoleBuilder:
			label += theme.IconBuilder + " builder"
		case models.RoleRunner:
			label += theme.IconRunner + " runner"
		}
	}
	if label == "" {
		return Styled("none", theme.Ghost)
	}
	return Styled(label, theme.Note)
}

func Health(health string) Cell {
	switch health {
	case "healthy":
		return Styled("healthy", theme.Good)
	case "unhealthy":
		return Styled("unhealthy", theme.Bad)
	case "starting":
		return Styled("starting", theme.Warn)
	default:
		return Styled("–", theme.Ghost)
	}
}
