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
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

const alertColumns = `id, agent_id, agent_name, type, message, severity, resolved, created_at, resolved_at`

func (s *Store) CreateAlert(alert *models.Alert) error {
	_, err := s.db.Exec(`
		INSERT INTO alerts (id, agent_id, agent_name, type, message, severity, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		alert.ID, alert.AgentID, alert.AgentName, alert.Type,
		alert.Message, alert.Severity, alert.CreatedAt)
	return err
}

func (s *Store) ResolveAlert(id string) error {
	_, err := s.db.Exec(`UPDATE alerts SET resolved = 1, resolved_at = ? WHERE id = ?`,
		time.Now(), id)
	return err
}

func (s *Store) ListActiveAlerts() ([]models.Alert, error) {
	return s.queryAlerts(`SELECT ` + alertColumns + `
		FROM alerts WHERE resolved = 0 ORDER BY created_at DESC`)
}

func (s *Store) ListRecentAlerts(limit int) ([]models.Alert, error) {
	return s.queryAlerts(`SELECT `+alertColumns+`
		FROM alerts ORDER BY created_at DESC LIMIT ?`, limit)
}

func (s *Store) queryAlerts(query string, args ...any) ([]models.Alert, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts := make([]models.Alert, 0)
	for rows.Next() {
		var alert models.Alert
		var resolvedAt sql.NullTime
		if err := rows.Scan(&alert.ID, &alert.AgentID, &alert.AgentName, &alert.Type,
			&alert.Message, &alert.Severity, &alert.Resolved,
			&alert.CreatedAt, &resolvedAt); err != nil {
			return nil, err
		}
		alert.ResolvedAt = timePointer(resolvedAt)
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}
