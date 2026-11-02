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
	"fmt"
	"time"

	"github.com/mustafanass/uruflow/internal/storage"
)

func (s *Store) ListConsoleSessions() ([]storage.ConsoleSession, error) {
	rows, err := s.db.Query(`SELECT token_hash,username,csrf,expires_at,last_active,credentials FROM console_sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []storage.ConsoleSession
	for rows.Next() {
		var item storage.ConsoleSession
		var hash, credentials []byte
		var expires, active int64
		if err = rows.Scan(&hash, &item.Username, &item.CSRF, &expires, &active, &credentials); err != nil {
			return nil, err
		}
		if len(hash) != 32 || len(credentials) != 32 {
			return nil, fmt.Errorf("invalid persisted session")
		}
		copy(item.Hash[:], hash)
		copy(item.Credentials[:], credentials)
		item.Expires = time.Unix(0, expires)
		item.LastActive = time.Unix(0, active)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) SaveConsoleSession(item storage.ConsoleSession) error {
	if _, err := s.db.Exec(`DELETE FROM console_sessions WHERE expires_at<=?`, time.Now().UnixNano()); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO console_sessions(token_hash,username,csrf,expires_at,last_active,credentials) VALUES(?,?,?,?,?,?) ON CONFLICT(token_hash) DO UPDATE SET last_active=excluded.last_active`, item.Hash[:], item.Username, item.CSRF, item.Expires.UnixNano(), item.LastActive.UnixNano(), item.Credentials[:])
	return err
}

func (s *Store) DeleteConsoleSession(hash [32]byte) error {
	_, err := s.db.Exec(`DELETE FROM console_sessions WHERE token_hash=?`, hash[:])
	return err
}

func (s *Store) DeleteConsoleUserSessions(username string) error {
	_, err := s.db.Exec(`DELETE FROM console_sessions WHERE username=?`, username)
	return err
}

func (s *Store) ResetConsoleAdministrator(username string, hash []byte) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var role string
	if err = tx.QueryRow(`SELECT role FROM console_users WHERE username=?`, username).Scan(&role); err != nil {
		return err
	}
	if role != "admin" {
		return fmt.Errorf("local recovery is restricted to administrator accounts")
	}
	if _, err = tx.Exec(`UPDATE console_users SET password_hash=? WHERE username=?`, hash, username); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM console_sessions WHERE username=?`, username); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO audit_entries(actor,action,resource,outcome) VALUES('local-admin','user.password.recover',?,'succeeded')`, username); err != nil {
		return err
	}
	return tx.Commit()
}
