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

import "github.com/urustack/uruflow/internal/models"

func (s *Store) AppendLog(line *models.LogLine) error {
	_, err := s.db.Exec(`
		INSERT INTO release_logs (release_id, stage, agent_name, stream, line, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)`,
		line.ReleaseID, line.Stage, line.AgentName, line.Stream, line.Line, line.Timestamp)
	return err
}

func (s *Store) ListLogs(releaseID string) ([]models.LogLine, error) {
	rows, err := s.db.Query(`
		SELECT id, release_id, stage, agent_name, stream, line, timestamp
		FROM release_logs WHERE release_id = ? ORDER BY id`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lines := make([]models.LogLine, 0)
	for rows.Next() {
		var line models.LogLine
		if err := rows.Scan(&line.ID, &line.ReleaseID, &line.Stage, &line.AgentName,
			&line.Stream, &line.Line, &line.Timestamp); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}
