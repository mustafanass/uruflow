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

func (s *Store) SetSecret(name string, value []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO secrets (name, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at`,
		name, value, time.Now())
	return err
}

func (s *Store) GetSecret(name string) ([]byte, error) {
	var value []byte

	err := s.db.QueryRow(`SELECT value FROM secrets WHERE name = ?`, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	return value, err
}

func (s *Store) ListSecrets() ([]models.Secret, error) {
	rows, err := s.db.Query(`SELECT name, created_at, updated_at FROM secrets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	secrets := make([]models.Secret, 0)
	for rows.Next() {
		var secret models.Secret
		if err := rows.Scan(&secret.Name, &secret.CreatedAt, &secret.UpdatedAt); err != nil {
			return nil, err
		}
		secrets = append(secrets, secret)
	}
	return secrets, rows.Err()
}

func (s *Store) DeleteSecret(name string) error {
	result, err := s.db.Exec(`DELETE FROM secrets WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}
