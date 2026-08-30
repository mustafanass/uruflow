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

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
)

const releaseColumns = `id, project, branch, commit_sha, commits, image, images, digest, status,
	builder, builder_name, trigger_type, message, spec, started_at, ended_at, duration_ms`

func (s *Store) CreateRelease(release *models.Release) error {
	_, err := s.db.Exec(`
		INSERT INTO releases (id, project, branch, commit_sha, commits, image, images, digest, status,
		                      builder, builder_name, trigger_type, message, spec, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		release.ID, release.Project, release.Branch, release.Commit, encodeJSON(release.Commits), release.Image,
		encodeJSON(release.Images), release.Digest, release.Status, release.Builder,
		release.BuilderName, release.Trigger, release.Message, encodeJSON(release.Spec), release.StartedAt)
	return err
}

func (s *Store) ClaimRelease(release *models.Release, targets []models.ReleaseTarget) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO releases (id, project, branch, commit_sha, commits, image, images, digest, status,
		                      builder, builder_name, trigger_type, message, spec, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		release.ID, release.Project, release.Branch, release.Commit, encodeJSON(release.Commits), release.Image,
		encodeJSON(release.Images), release.Digest, release.Status, release.Builder,
		release.BuilderName, release.Trigger, release.Message, encodeJSON(release.Spec), release.StartedAt); err != nil {
		return err
	}

	for index := range targets {
		target := &targets[index]
		if _, err := tx.Exec(`
			INSERT INTO release_targets (release_id, agent_id, agent_name, status, message, ended_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			target.ReleaseID, target.AgentID, target.AgentName, target.Status,
			target.Message, nullTime(target.EndedAt)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) UpdateRelease(release *models.Release) error {
	_, err := s.db.Exec(`
		UPDATE releases
		SET commit_sha = ?, commits = ?, image = ?, images = ?, digest = ?, status = ?, message = ?,
		    ended_at = ?, duration_ms = ?
		WHERE id = ?`,
		release.Commit, encodeJSON(release.Commits), release.Image, encodeJSON(release.Images), release.Digest,
		release.Status, release.Message, nullTime(release.EndedAt), release.Duration, release.ID)
	return err
}

func (s *Store) GetRelease(id string) (*models.Release, error) {
	release, err := scanRelease(s.db.QueryRow(`SELECT `+releaseColumns+` FROM releases WHERE id = ?`, id))
	if err != nil {
		return nil, err
	}

	targets, err := s.ListReleaseTargets(id)
	if err != nil {
		return nil, err
	}
	release.Targets = targets
	return release, nil
}

func (s *Store) ListReleases(limit int) ([]models.Release, error) {
	return s.queryReleases(`SELECT `+releaseColumns+`
		FROM releases ORDER BY started_at DESC LIMIT ?`, limit)
}

func (s *Store) ListActiveReleases() ([]models.Release, error) {
	return s.queryReleases(`SELECT `+releaseColumns+`
		FROM releases WHERE status NOT IN (?, ?, ?) ORDER BY started_at`,
		models.StatusSucceeded, models.StatusFailed, models.StatusSkipped)
}

func (s *Store) ListReleasesByProject(project string, limit int) ([]models.Release, error) {
	return s.queryReleases(`SELECT `+releaseColumns+`
		FROM releases WHERE project = ? ORDER BY started_at DESC LIMIT ?`, project, limit)
}

func (s *Store) ProjectHasActiveRelease(project string) (bool, error) {
	var active bool
	err := s.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM releases WHERE project = ? AND status NOT IN (?, ?, ?)
	)`, project, models.StatusSucceeded, models.StatusFailed, models.StatusSkipped).Scan(&active)
	return active, err
}

func (s *Store) LastSuccessfulRelease(project string) (*models.Release, error) {
	return scanRelease(s.db.QueryRow(`SELECT `+releaseColumns+`
		FROM releases
		WHERE project = ? AND status = ? AND image != ''
		ORDER BY started_at DESC LIMIT 1`, project, models.StatusSucceeded))
}

func (s *Store) SaveReleaseTarget(target *models.ReleaseTarget) error {
	_, err := s.db.Exec(`
		INSERT INTO release_targets (release_id, agent_id, agent_name, status, message, ended_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(release_id, agent_id) DO UPDATE SET
			agent_name = excluded.agent_name,
			status = excluded.status,
			message = excluded.message,
			ended_at = excluded.ended_at`,
		target.ReleaseID, target.AgentID, target.AgentName, target.Status,
		target.Message, nullTime(target.EndedAt))
	return err
}

func (s *Store) ListReleaseTargets(releaseID string) ([]models.ReleaseTarget, error) {
	rows, err := s.db.Query(`
		SELECT release_id, agent_id, agent_name, status, message, ended_at
		FROM release_targets WHERE release_id = ? ORDER BY agent_name`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make([]models.ReleaseTarget, 0)
	for rows.Next() {
		var target models.ReleaseTarget
		var endedAt sql.NullTime
		if err := rows.Scan(&target.ReleaseID, &target.AgentID, &target.AgentName,
			&target.Status, &target.Message, &endedAt); err != nil {
			return nil, err
		}
		target.EndedAt = timePointer(endedAt)
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *Store) queryReleases(query string, args ...any) ([]models.Release, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	releases := make([]models.Release, 0)
	for rows.Next() {
		release, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		releases = append(releases, *release)
	}
	return releases, rows.Err()
}

func scanRelease(row scanner) (*models.Release, error) {
	var release models.Release
	var endedAt sql.NullTime
	var commits, images, spec string

	err := row.Scan(&release.ID, &release.Project, &release.Branch, &release.Commit, &commits,
		&release.Image, &images, &release.Digest, &release.Status, &release.Builder,
		&release.BuilderName, &release.Trigger, &release.Message, &spec,
		&release.StartedAt, &endedAt, &release.Duration)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	decodeJSON(images, &release.Images)
	decodeJSON(commits, &release.Commits)
	decodeJSON(spec, &release.Spec)
	release.EndedAt = timePointer(endedAt)
	return &release, nil
}
