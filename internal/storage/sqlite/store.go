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
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
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
	{table: "release_targets", name: "containers", spec: "TEXT NOT NULL DEFAULT '{}'"},
	{table: "releases", name: "artifacts_expired", spec: "INTEGER NOT NULL DEFAULT 0"},
	{table: "console_users", name: "projects", spec: "TEXT NOT NULL DEFAULT '[]'"},
	{table: "alerts", name: "project", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "agents", name: "metrics_detail", spec: "TEXT NOT NULL DEFAULT '{}'"},
	{table: "containers", name: "observation", spec: "TEXT NOT NULL DEFAULT '{}'"},
	{table: "projects", name: "env", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "projects", name: "source", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "projects", name: "services", spec: "TEXT NOT NULL DEFAULT '[]'"},
	{table: "projects", name: "workflow", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "projects", name: "timeout_ns", spec: "INTEGER NOT NULL DEFAULT 0"},
	{table: "projects", name: "resources", spec: "TEXT NOT NULL DEFAULT '{}'"},
	{table: "releases", name: "images", spec: "TEXT NOT NULL DEFAULT '{}'"},
	{table: "releases", name: "commits", spec: "TEXT NOT NULL DEFAULT '{}'"},
	{table: "releases", name: "spec", spec: "TEXT NOT NULL DEFAULT '{}'"},
	{table: "containers", name: "service", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "webhook_deliveries", name: "repository", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "webhook_deliveries", name: "branch", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "webhook_deliveries", name: "commit_sha", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "webhook_deliveries", name: "result", spec: "TEXT NOT NULL DEFAULT ''"},
	{table: "webhook_deliveries", name: "detail", spec: "TEXT NOT NULL DEFAULT ''"},
}

func New(path string) (*Store, error) {

	if path != ":memory:" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("open private database: %w", err)
		}
		if err = file.Chmod(0600); err != nil {
			file.Close()
			return nil, fmt.Errorf("secure database permissions: %w", err)
		}
		if err = file.Close(); err != nil {
			return nil, err
		}
	}
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

func (s *Store) ClaimWebhookDelivery(provider, deliveryID string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM webhook_deliveries WHERE received_at < datetime('now', '-30 days')`); err != nil {
		return false, err
	}
	result, err := tx.Exec(`INSERT OR IGNORE INTO webhook_deliveries (provider, delivery_id) VALUES (?, ?)`,
		provider, deliveryID)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return rows == 1, nil
}

func (s *Store) RecordWebhookDelivery(delivery models.WebhookDelivery) error {
	_, err := s.db.Exec(`UPDATE webhook_deliveries SET repository = ?, branch = ?, commit_sha = ?, result = ?, detail = ?
		WHERE provider = ? AND delivery_id = ?`,
		delivery.Repository, delivery.Branch, delivery.Commit, delivery.Result, delivery.Detail,
		delivery.Provider, delivery.DeliveryID)
	return err
}

func (s *Store) ListWebhookDeliveries(limit int) ([]models.WebhookDelivery, error) {
	rows, err := s.db.Query(`SELECT provider, delivery_id, received_at, repository, branch, commit_sha, result, detail
		FROM webhook_deliveries ORDER BY received_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	deliveries := make([]models.WebhookDelivery, 0)
	for rows.Next() {
		var delivery models.WebhookDelivery
		if err := rows.Scan(&delivery.Provider, &delivery.DeliveryID, &delivery.ReceivedAt, &delivery.Repository,
			&delivery.Branch, &delivery.Commit, &delivery.Result, &delivery.Detail); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, rows.Err()
}

func (s *Store) ListPushSources() ([]models.PushSource, error) {
	rows, err := s.db.Query(`SELECT project, repository, branch FROM push_sources ORDER BY project, repository, branch`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := make([]models.PushSource, 0)
	for rows.Next() {
		var source models.PushSource
		if err := rows.Scan(&source.Project, &source.Repository, &source.Branch); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Store) SetPushSource(source models.PushSource, enabled bool) error {
	if !enabled {
		_, err := s.db.Exec(`DELETE FROM push_sources WHERE project = ? AND repository = ? AND branch = ?`,
			source.Project, source.Repository, source.Branch)
		return err
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO push_sources (project, repository, branch) VALUES (?, ?, ?)`,
		source.Project, source.Repository, source.Branch)
	return err
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
