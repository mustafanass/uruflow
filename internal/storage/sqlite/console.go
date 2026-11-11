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
	"slices"

	"github.com/mustafanass/uruflow/internal/storage"
)

var _ storage.ConsoleStore = (*Store)(nil)

func (s *Store) ListConsoleUsers() ([]storage.ConsoleUser, error) {
	rows, err := s.db.Query(`SELECT username,role,created_at,projects FROM console_users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []storage.ConsoleUser{}
	for rows.Next() {
		var u storage.ConsoleUser
		var projects string
		if err = rows.Scan(&u.Username, &u.Role, &u.CreatedAt, &projects); err != nil {
			return nil, err
		}
		decodeJSON(projects, &u.Projects)
		if u.Projects == nil {
			u.Projects = []string{}
		}
		result = append(result, u)
	}
	return result, rows.Err()
}
func (s *Store) GetConsoleUser(name string) (*storage.ConsoleUser, error) {
	var u storage.ConsoleUser
	var projects string
	err := s.db.QueryRow(`SELECT username,role,password_hash,created_at,projects FROM console_users WHERE username=?`, name).Scan(&u.Username, &u.Role, &u.PasswordHash, &u.CreatedAt, &projects)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	decodeJSON(projects, &u.Projects)
	if u.Projects == nil {
		u.Projects = []string{}
	}
	return &u, err
}
func (s *Store) BootstrapConsoleUser(u *storage.ConsoleUser) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRow(`SELECT count(*) FROM console_users`).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if _, err = tx.Exec(`INSERT INTO console_users(username,role,password_hash) VALUES (?,'admin',?)`, u.Username, u.PasswordHash); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
func (s *Store) CreateConsoleUser(u *storage.ConsoleUser) error {
	_, err := s.db.Exec(`INSERT INTO console_users(username,role,password_hash,projects) VALUES (?,?,?,?)`, u.Username, u.Role, u.PasswordHash, encodeJSON(u.Projects))
	return err
}
func (s *Store) DeleteConsoleUser(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var role string
	if err = tx.QueryRow(`SELECT role FROM console_users WHERE username=?`, name).Scan(&role); err != nil {
		return err
	}
	if role == "admin" {
		var admins int
		if err = tx.QueryRow(`SELECT count(*) FROM console_users WHERE role='admin'`).Scan(&admins); err != nil {
			return err
		}
		if admins <= 1 {
			return errors.New("the last administrator cannot be removed")
		}
	}
	if _, err = tx.Exec(`DELETE FROM console_users WHERE username=?`, name); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) AppendAudit(a *storage.AuditEntry) error {
	_, err := s.db.Exec(`INSERT INTO audit_entries(actor,action,resource,outcome) VALUES (?,?,?,?)`, a.Actor, a.Action, a.Resource, a.Outcome)
	return err
}
func (s *Store) ListAudit(limit int) ([]storage.AuditEntry, error) {
	rows, err := s.db.Query(`SELECT id,actor,action,resource,outcome,created_at FROM audit_entries ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []storage.AuditEntry{}
	for rows.Next() {
		var a storage.AuditEntry
		if err = rows.Scan(&a.ID, &a.Actor, &a.Action, &a.Resource, &a.Outcome, &a.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (s *Store) ChangeConsolePassword(name string, previous, next []byte) error {
	result, err := s.db.Exec(`UPDATE console_users SET password_hash=? WHERE username=? AND password_hash=?`, next, name, previous)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("password changed concurrently; sign in again")
	}
	return nil
}
func (s *Store) ChangeConsoleRole(name, role string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current string
	if err = tx.QueryRow(`SELECT role FROM console_users WHERE username=?`, name).Scan(&current); err != nil {
		return err
	}
	if current == "admin" && role != "admin" {
		var count int
		if err = tx.QueryRow(`SELECT count(*) FROM console_users WHERE role='admin'`).Scan(&count); err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("the last administrator cannot be demoted")
		}
	}
	if _, err = tx.Exec(`UPDATE console_users SET role=? WHERE username=?`, role, name); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ChangeConsoleProjects(name string, projects []string) error {
	result, err := s.db.Exec(`UPDATE console_users SET projects=? WHERE username=?`, encodeJSON(projects), name)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func removeProjectGrants(tx *sql.Tx, project string) error {
	rows, err := tx.Query(`SELECT username, projects FROM console_users`)
	if err != nil {
		return err
	}
	granted := map[string][]string{}
	for rows.Next() {
		var username, encoded string
		if err = rows.Scan(&username, &encoded); err != nil {
			rows.Close()
			return err
		}
		var projects []string
		decodeJSON(encoded, &projects)
		if slices.Contains(projects, project) {
			granted[username] = slices.DeleteFunc(projects, func(name string) bool { return name == project })
		}
	}
	if err = rows.Close(); err != nil {
		return err
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for username, projects := range granted {
		if _, err = tx.Exec(`UPDATE console_users SET projects=? WHERE username=?`, encodeJSON(projects), username); err != nil {
			return err
		}
	}
	return nil
}
