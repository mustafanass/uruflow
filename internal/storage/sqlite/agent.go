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
	"errors"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
)

const agentColumns = `id, name, auth_key, roles, host, hostname, version, platform, status,
	cpu_percent, memory_percent, memory_used, memory_total,
	disk_percent, disk_used, disk_total, uptime, last_seen, registered_at`

func (s *Store) CreateAgent(agent *models.Agent) error {
	_, err := s.db.Exec(`
		INSERT INTO agents (id, name, auth_key, roles, status, registered_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		agent.ID, agent.Name, agent.Key, encodeJSON(agent.Roles),
		models.AgentOffline, time.Now())
	return err
}

func (s *Store) UpdateAgent(agent *models.Agent) error {
	_, err := s.db.Exec(`
		UPDATE agents
		SET name = ?, roles = ?, host = ?, hostname = ?, version = ?,
		    platform = ?, status = ?, last_seen = ?
		WHERE id = ?`,
		agent.Name, encodeJSON(agent.Roles), agent.Host, agent.Hostname,
		agent.Version, agent.Platform, agent.Status, agent.LastSeen, agent.ID)
	return err
}

func (s *Store) SetAgentStatus(id string, status models.AgentStatus) error {
	_, err := s.db.Exec(`UPDATE agents SET status = ?, last_seen = ? WHERE id = ?`,
		status, time.Now(), id)
	return err
}

func (s *Store) SetAgentMetrics(id string, metrics *models.Metrics) error {
	_, err := s.db.Exec(`
		UPDATE agents
		SET cpu_percent = ?, memory_percent = ?, memory_used = ?, memory_total = ?,
		    disk_percent = ?, disk_used = ?, disk_total = ?, uptime = ?, last_seen = ?
		WHERE id = ?`,
		metrics.CPUPercent, metrics.MemoryPercent, metrics.MemoryUsed, metrics.MemoryTotal,
		metrics.DiskPercent, metrics.DiskUsed, metrics.DiskTotal, metrics.Uptime,
		time.Now(), id)
	return err
}

func (s *Store) GetAgent(id string) (*models.Agent, error) {
	return s.scanAgent(s.db.QueryRow(`SELECT `+agentColumns+` FROM agents WHERE id = ?`, id))
}

func (s *Store) GetAgentByName(name string) (*models.Agent, error) {
	return s.scanAgent(s.db.QueryRow(`SELECT `+agentColumns+` FROM agents WHERE name = ?`, name))
}

func (s *Store) ListAgents() ([]models.Agent, error) {
	rows, err := s.db.Query(`SELECT ` + agentColumns + ` FROM agents ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]models.Agent, 0)
	for rows.Next() {
		agent, err := s.scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, *agent)
	}
	return agents, rows.Err()
}

func (s *Store) DeleteAgent(id string) error {
	result, err := s.db.Exec(`DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanAgent(row scanner) (*models.Agent, error) {
	var agent models.Agent
	var roles string
	var metrics models.Metrics
	var lastSeen sql.NullTime

	err := row.Scan(
		&agent.ID, &agent.Name, &agent.Key, &roles, &agent.Host, &agent.Hostname,
		&agent.Version, &agent.Platform, &agent.Status,
		&metrics.CPUPercent, &metrics.MemoryPercent, &metrics.MemoryUsed, &metrics.MemoryTotal,
		&metrics.DiskPercent, &metrics.DiskUsed, &metrics.DiskTotal, &metrics.Uptime,
		&lastSeen, &agent.RegisteredAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	decodeJSON(roles, &agent.Roles)
	agent.LastSeen = timeValue(lastSeen)
	agent.Metrics = &metrics
	return &agent, nil
}
