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
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func (s *Store) SetAllAgentsOffline() error {
	_, err := s.db.Exec(`UPDATE agents SET status = ? WHERE status != ?`,
		models.AgentOffline, models.AgentOffline)
	return err
}

func (s *Store) FailUnfinishedReleases(message string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	terminal := []any{models.StatusSucceeded, models.StatusFailed, models.StatusSkipped}

	if _, err := tx.Exec(`
		UPDATE release_targets
		SET status = ?, message = ?, ended_at = ?
		WHERE status NOT IN (?, ?, ?)`,
		append([]any{models.StatusFailed, message, now}, terminal...)...); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE releases
		SET status = ?, message = ?, ended_at = ?,
		    duration_ms = CAST((julianday(?) - julianday(started_at)) * 86400000 AS INTEGER)
		WHERE status NOT IN (?, ?, ?)`,
		append([]any{models.StatusFailed, message, now, now}, terminal...)...); err != nil {
		return err
	}

	return tx.Commit()
}
