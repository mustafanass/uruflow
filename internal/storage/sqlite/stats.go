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

package sqlite

import (
	"github.com/urustack/uruflow/internal/models"
	"github.com/urustack/uruflow/internal/storage"
)

func (s *Store) Stats() (*storage.Stats, error) {
	stats := &storage.Stats{}

	row := s.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM agents),
			(SELECT COUNT(*) FROM agents WHERE status = ?),
			(SELECT COUNT(*) FROM projects),
			(SELECT COUNT(*) FROM releases),
			(SELECT COUNT(*) FROM releases WHERE started_at >= date('now')),
			(SELECT COUNT(*) FROM containers WHERE state = 'running'),
			(SELECT COUNT(*) FROM containers WHERE state != 'running'),
			(SELECT COUNT(*) FROM alerts WHERE resolved = 0)`,
		models.AgentOnline)

	if err := row.Scan(&stats.AgentsTotal, &stats.AgentsOnline, &stats.ProjectsTotal,
		&stats.ReleasesTotal, &stats.ReleasesToday, &stats.ContainersRunning,
		&stats.ContainersStopped, &stats.AlertsActive); err != nil {
		return nil, err
	}

	var finished, succeeded int
	if err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(status = ?), 0)
		FROM releases WHERE status IN (?, ?)`,
		models.StatusSucceeded, models.StatusSucceeded, models.StatusFailed,
	).Scan(&finished, &succeeded); err != nil {
		return nil, err
	}
	if finished > 0 {
		stats.SuccessRate = float64(succeeded) / float64(finished) * 100
	}

	agents, err := s.ListAgents()
	if err != nil {
		return nil, err
	}
	for _, agent := range agents {
		if agent.Status != models.AgentOnline {
			continue
		}
		if agent.HasRole(models.RoleBuilder) {
			stats.BuildersOnline++
		}
		if agent.HasRole(models.RoleRunner) {
			stats.RunnersOnline++
		}
	}

	return stats, nil
}
