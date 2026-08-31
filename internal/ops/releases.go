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
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/models"
)

const releaseLogPageSize = 500

func (e *Engine) releases(ctx context.Context, args []string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		limit := 25
		if len(args) == 3 && args[1] == "--limit" {
			parsed, err := strconv.Atoi(args[2])
			if err != nil || parsed < 1 || parsed > 1000 {
				return errors.New("--limit must be between 1 and 1000")
			}
			limit = parsed
		}
		values, err := e.server.Store().ListReleases(limit)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, release := range values {
			rows = append(rows, []string{release.ID, release.Project, string(release.Status),
				string(release.Trigger), short(release.Commit, 8), duration(release.Duration), since(release.StartedAt)})
		}
		return emit(Table("releases", []string{"ID", "PROJECT", "STATE", "TRIGGER", "REVISION", "DURATION", "AGE"}, rows))
	}
	if len(args) < 2 {
		return grammar.GroupUsageError("release")
	}
	switch args[0] {
	case "show":
		release, err := e.server.Store().GetRelease(args[1])
		if err != nil {
			return err
		}
		if err := emit(Table("release "+release.ID, []string{"FIELD", "VALUE"}, [][]string{
			{"project", release.Project}, {"state", string(release.Status)}, {"trigger", string(release.Trigger)},
			{"branch", release.Branch}, {"revision", release.Commit}, {"image", release.Image},
			{"duration", duration(release.Duration)}, {"started", release.StartedAt.Local().Format(time.RFC3339)},
			{"message", release.Message},
		})); err != nil {
			return err
		}
		rows := make([][]string, 0, len(release.Targets))
		for _, target := range release.Targets {
			rows = append(rows, []string{target.AgentName, string(target.Status), target.Message})
		}
		return emit(Table("targets", []string{"AGENT", "STATE", "DETAIL"}, rows))
	case "follow":
		return e.followRelease(ctx, args[1], emit)
	case "logs":
		if contains(args[2:], "--follow") {
			return e.followRelease(ctx, args[1], emit)
		}
		return e.emitLogs(args[1], 0, emit)
	default:
		return fmt.Errorf("unknown release command %q", args[0])
	}
}

func (e *Engine) emitLogs(releaseID string, after int64, emit Emit) error {
	_, err := e.emitLogsAfter(releaseID, after, emit)
	return err
}

func (e *Engine) emitLogsAfter(releaseID string, after int64, emit Emit) (int64, error) {
	cursor := after
	for {
		logs, err := e.server.Store().ListLogs(releaseID, cursor, releaseLogPageSize)
		if err != nil {
			return cursor, err
		}
		for _, line := range logs {
			level := "info"
			if line.Stream == "stderr" {
				level = "warning"
			}
			if err := emit(Event{Type: EventLog, Time: line.Timestamp, Level: level, Operation: releaseID,
				Title: line.AgentName, Message: line.Line, Data: map[string]any{"id": line.ID, "stage": line.Stage, "stream": line.Stream}}); err != nil {
				return cursor, err
			}
			cursor = line.ID
		}
		if len(logs) < releaseLogPageSize {
			return cursor, nil
		}
	}
}

func (e *Engine) followRelease(ctx context.Context, releaseID string, emit Emit) error {
	var lastLog int64
	var lastStatus models.Status
	ticker := time.NewTicker(followInterval)
	defer ticker.Stop()
	for {
		var err error
		lastLog, err = e.emitLogsAfter(releaseID, lastLog, emit)
		if err != nil {
			return err
		}
		release, err := e.server.Store().GetRelease(releaseID)
		if err != nil {
			return err
		}
		if release.Status != lastStatus {
			lastStatus = release.Status
			level := "info"
			if release.Status == models.StatusSucceeded {
				level = "success"
			} else if release.Status == models.StatusFailed {
				level = "error"
			}
			message := fmt.Sprintf("%s · %s", release.Project, release.Status)
			if release.Message != "" {
				message += " · " + release.Message
			}
			if err := emit(Event{Type: EventMessage, Time: time.Now(), Level: level, Operation: releaseID, Message: message}); err != nil {
				return err
			}
		}
		if release.Status.Done() {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
