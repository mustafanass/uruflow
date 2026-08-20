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
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urustack/uruflow/internal/storage"
)

var _ storage.Store = (*Store)(nil)

type Store struct {
	db *sql.DB
}

type column struct {
	table string
	name  string
	spec  string
}

var addedColumns = []column{
	{table: "projects", name: "env", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "projects", name: "source", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "projects", name: "services", spec: "TEXT NOT NULL DEFAULT '[]'"},
	{table: "releases", name: "images", spec: "TEXT NOT NULL DEFAULT '{}'"},
	{table: "containers", name: "service", spec: "TEXT NOT NULL DEFAULT ''"},
}

func New(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_foreign_keys=on", path)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", filepath.Base(path), err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	store := &Store{db: db}
	if err := store.addMissingColumns(); err != nil {
		db.Close()
		return nil, fmt.Errorf("upgrade schema: %w", err)
	}

	return store, nil
}

func (s *Store) addMissingColumns() error {
	for _, column := range addedColumns {
		var present int
		row := s.db.QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, column.table, column.name)
		if err := row.Scan(&present); err != nil {
			return err
		}
		if present > 0 {
			continue
		}
		if _, err := s.db.Exec(
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, column.table, column.name, column.spec)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func encodeJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(data)
}

func decodeJSON(data string, target any) {
	if data == "" {
		return
	}
	json.Unmarshal([]byte(data), target)
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func timeValue(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time
	}
	return time.Time{}
}

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	moment := value.Time
	return &moment
}
