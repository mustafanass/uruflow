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
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/observe"
)

func (e *Engine) Release(id string) (*models.Release, error) {
	r, err := e.server.Store().GetRelease(id)
	if err != nil {
		return nil, err
	}
	r.Spec = observe.RedactProject(r.Spec)
	return r, nil
}
func (e *Engine) ProjectReleases(name string) ([]models.Release, error) {
	rows, err := e.server.Store().ProjectReleaseSummaries(name)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Spec = models.Project{}
	}
	return rows, nil
}
func (e *Engine) FollowReleaseLogs(ctx context.Context, id string, after int64, emit func(models.LogLine) error) error {
	if _, err := e.server.Store().GetRelease(id); err != nil {
		return err
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		for {
			rows, err := e.server.Store().ListLogs(id, after, 500)
			if err != nil {
				return err
			}
			for _, line := range rows {
				if err := emit(line); err != nil {
					return err
				}
				after = line.ID
			}
			if len(rows) < 500 {
				break
			}
		}
		r, err := e.server.Store().GetRelease(id)
		if err != nil {
			return err
		}
		if r.Status.Done() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
