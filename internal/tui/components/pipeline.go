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
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

const stepLink = "──"

type Step struct {
	Label  string
	Status models.Status
}

func Steps(steps []Step) string {
	parts := make([]string, 0, len(steps)*2)

	for index, step := range steps {
		if index > 0 {
			parts = append(parts, theme.Rule.Render(stepLink))
		}
		parts = append(parts, stepGlyph(step)+" "+stepLabel(step))
	}

	return strings.Join(parts, " ")
}

func ReleaseSteps(release *models.Release) []Step {
	build := models.StatusPending
	rollout := models.StatusPending

	switch release.Status {
	case models.StatusBuilding:
		build = models.StatusBuilding
	case models.StatusReleasing:
		build = models.StatusSucceeded
		rollout = models.StatusReleasing
	case models.StatusSucceeded:
		build = models.StatusSucceeded
		rollout = models.StatusSucceeded
	case models.StatusFailed:
		if release.Image == "" {
			build = models.StatusFailed
		} else {
			build = models.StatusSucceeded
			rollout = models.StatusFailed
		}
	}

	if release.Trigger == models.TriggerRollback {
		build = models.StatusSkipped
	}

	return []Step{
		{Label: "build", Status: build},
		{Label: "push", Status: pushStatus(release, build)},
		{Label: "release", Status: rollout},
	}
}

func pushStatus(release *models.Release, build models.Status) models.Status {
	if release.Trigger == models.TriggerRollback {
		return models.StatusSkipped
	}
	if build == models.StatusSucceeded && release.Image != "" {
		return models.StatusSucceeded
	}
	if build == models.StatusFailed {
		return models.StatusSkipped
	}
	return models.StatusPending
}

func stepGlyph(step Step) string {
	style := StatusStyle(step.Status)

	switch step.Status {
	case models.StatusSucceeded:
		return style.Render(theme.IconStepDone)
	case models.StatusBuilding, models.StatusReleasing:
		return style.Render(theme.IconStepLive)
	case models.StatusFailed:
		return style.Render(theme.IconFailure)
	case models.StatusSkipped:
		return style.Render(theme.IconSkipped)
	default:
		return style.Render(theme.IconStepTodo)
	}
}

func stepLabel(step Step) string {
	switch step.Status {
	case models.StatusPending, models.StatusSkipped:
		return theme.Ghost.Render(step.Label)
	default:
		return StatusStyle(step.Status).Render(step.Label)
	}
}
