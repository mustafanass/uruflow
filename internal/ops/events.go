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
	"sort"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func (e *Engine) events(ctx context.Context, emit Emit) error {
	if err := emit(Message("success", "following server activity")); err != nil {
		return err
	}
	statuses := make(map[string]models.Status)
	logs := make(map[string]int64)
	alerts := make(map[string]bool)
	initial := true
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	for {
		releases, err := e.server.Store().ListReleases(50)
		if err != nil {
			return err
		}
		for _, release := range releases {
			lines, err := e.server.Store().ListLogs(release.ID)
			if err != nil {
				return err
			}
			sort.Slice(lines, func(i, j int) bool { return lines[i].ID < lines[j].ID })
			// A fleet stream starts at "now". Seed completed history without
			// replaying every stored build line into a fresh workspace.
			if initial && release.Status.Done() {
				statuses[release.ID] = release.Status
				if len(lines) > 0 {
					logs[release.ID] = lines[len(lines)-1].ID
				}
				continue
			}
			if previous, found := statuses[release.ID]; !found || previous != release.Status {
				statuses[release.ID] = release.Status
				level := "info"
				if release.Status == models.StatusSucceeded {
					level = "success"
				} else if release.Status == models.StatusFailed {
					level = "error"
				}
				if err := emit(Event{Type: EventMessage, Time: time.Now(), Level: level, Operation: release.ID,
					Message: release.Project + " · " + string(release.Status)}); err != nil {
					return err
				}
			}
			for _, line := range lines {
				if line.ID <= logs[release.ID] {
					continue
				}
				logs[release.ID] = line.ID
				level := "info"
				if line.Stream == "stderr" {
					level = "warning"
				}
				if err := emit(Event{Type: EventLog, Time: line.Timestamp, Level: level, Operation: release.ID,
					Title: line.AgentName, Message: line.Line}); err != nil {
					return err
				}
			}
		}
		activeAlerts, err := e.server.Store().ListActiveAlerts()
		if err != nil {
			return err
		}
		for _, alert := range activeAlerts {
			if alerts[alert.ID] {
				continue
			}
			alerts[alert.ID] = true
			level := "warning"
			if alert.Severity == models.SeverityCritical {
				level = "error"
			}
			if err := emit(Event{Type: EventMessage, Time: alert.CreatedAt, Level: level, Operation: alert.ID,
				Message: alert.AgentName + " · " + alert.Message}); err != nil {
				return err
			}
		}
		initial = false
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
