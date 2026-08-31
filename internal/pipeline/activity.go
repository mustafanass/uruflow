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

package pipeline

import (
	"time"

	"github.com/mustafanass/uruflow/internal/activity"
	"github.com/mustafanass/uruflow/internal/models"
)

func (p *Pipeline) publishRelease(release *models.Release) {
	if p.activity == nil || release == nil {
		return
	}
	level := "info"
	if release.Status == models.StatusSucceeded {
		level = "success"
	} else if release.Status == models.StatusFailed {
		level = "error"
	}
	message := release.Project + " · " + string(release.Status)
	if release.Message != "" {
		message += " · " + release.Message
	}
	p.activity.Publish(activity.Entry{Kind: activity.KindMessage, Time: time.Now(), Level: level,
		Operation: release.ID, Source: release.Project, Message: message})
}

func (p *Pipeline) publishLog(line *models.LogLine) {
	if p.activity == nil || line == nil {
		return
	}
	level := "info"
	if line.Stream == "stderr" {
		level = "warning"
	}
	p.activity.Publish(activity.Entry{Kind: activity.KindLog, Time: line.Timestamp, Level: level,
		Operation: line.ReleaseID, Source: line.AgentName, Message: line.Line})
}
