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
	"database/sql"

	"github.com/urustack/uruflow/internal/models"
)

const containerColumns = `id, agent_id, name, project, service, image, state, health,
	cpu_percent, memory_usage, memory_limit, network_rx, network_tx, restart_count, started_at`

func (s *Store) UpsertContainer(container *models.Container) error {
	_, err := s.db.Exec(`
		INSERT INTO containers (id, agent_id, name, project, service, image, state, health,
		                        cpu_percent, memory_usage, memory_limit,
		                        network_rx, network_tx, restart_count, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_id, id) DO UPDATE SET
			name = excluded.name,
			project = excluded.project,
			service = excluded.service,
			image = excluded.image,
			state = excluded.state,
			health = excluded.health,
			cpu_percent = excluded.cpu_percent,
			memory_usage = excluded.memory_usage,
			memory_limit = excluded.memory_limit,
			network_rx = excluded.network_rx,
			network_tx = excluded.network_tx,
			restart_count = excluded.restart_count,
			started_at = excluded.started_at`,
		container.ID, container.AgentID, container.Name, container.Project, container.Service,
		container.Image, container.State, container.Health, container.CPUPercent, container.MemoryUsage,
		container.MemoryLimit, container.NetworkRx, container.NetworkTx,
		container.RestartCount, container.StartedAt)
	return err
}

func (s *Store) ReplaceContainers(agentID string, containers []models.Container) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM containers WHERE agent_id = ?`, agentID); err != nil {
		return err
	}

	statement, err := tx.Prepare(`
		INSERT INTO containers (` + containerColumns + `)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer statement.Close()

	for _, container := range containers {
		if _, err := statement.Exec(container.ID, agentID, container.Name, container.Project,
			container.Service, container.Image, container.State, container.Health, container.CPUPercent,
			container.MemoryUsage, container.MemoryLimit, container.NetworkRx,
			container.NetworkTx, container.RestartCount, container.StartedAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListContainers() ([]models.Container, error) {
	return s.queryContainers(`SELECT ` + containerColumns + ` FROM containers ORDER BY name`)
}

func (s *Store) ListContainersByAgent(agentID string) ([]models.Container, error) {
	return s.queryContainers(`SELECT `+containerColumns+`
		FROM containers WHERE agent_id = ? ORDER BY name`, agentID)
}

func (s *Store) queryContainers(query string, args ...any) ([]models.Container, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	containers := make([]models.Container, 0)
	for rows.Next() {
		var container models.Container
		var startedAt sql.NullTime
		if err := rows.Scan(&container.ID, &container.AgentID, &container.Name,
			&container.Project, &container.Service, &container.Image, &container.State, &container.Health,
			&container.CPUPercent, &container.MemoryUsage, &container.MemoryLimit,
			&container.NetworkRx, &container.NetworkTx, &container.RestartCount,
			&startedAt); err != nil {
			return nil, err
		}
		container.StartedAt = timeValue(startedAt)
		containers = append(containers, container)
	}
	return containers, rows.Err()
}
