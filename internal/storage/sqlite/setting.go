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

func (s *Store) GetSetting(name string) (*models.Setting, error) {
	setting := &models.Setting{Name: name}
	err := s.db.QueryRow(`SELECT value, revision, updated_at FROM settings WHERE name = ?`, name).
		Scan(&setting.Value, &setting.Revision, &setting.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *Store) SaveSetting(setting *models.Setting) error {
	now := time.Now().UTC()
	var result sql.Result
	var err error
	if setting.Revision == 0 {
		result, err = s.db.Exec(`INSERT OR IGNORE INTO settings (name, value, revision, updated_at) VALUES (?, ?, 1, ?)`,
			setting.Name, setting.Value, now)
	} else {
		result, err = s.db.Exec(`UPDATE settings SET value = ?, revision = revision + 1, updated_at = ? WHERE name = ? AND revision = ?`,
			setting.Value, now, setting.Name, setting.Revision)
	}
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return storage.ErrConflict
	}
	setting.Revision++
	setting.UpdatedAt = now
	return nil
}
