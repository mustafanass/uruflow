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
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
)

const snapshotReleaseSelection = `id IN (SELECT id FROM releases ORDER BY started_at DESC LIMIT 100)
 OR id IN (SELECT id FROM (SELECT id, ROW_NUMBER() OVER(PARTITION BY project ORDER BY started_at DESC) AS rank FROM releases) WHERE rank=1)
 OR status IN ('pending','building','releasing')`

func (s *Store) ReleaseRevision() (int64, error) {
	var revision int64
	err := s.db.QueryRow(`SELECT revision FROM observation_revisions WHERE name='releases'`).Scan(&revision)
	return revision, err
}

func (s *Store) SnapshotReleases() ([]models.Release, error) {
	return s.releaseSummaries(snapshotReleaseSelection)
}

func (s *Store) ProjectReleaseSummaries(project string) ([]models.Release, error) {
	return s.releaseSummaries(`id IN (SELECT id FROM releases WHERE project=? ORDER BY started_at DESC LIMIT 100)
 OR id IN (SELECT id FROM releases WHERE project=? AND status='succeeded' AND (image<>'' OR EXISTS (SELECT 1 FROM json_each(images))) ORDER BY started_at DESC LIMIT 2)`, project, project)
}

func (s *Store) releaseSummaries(selection string, args ...any) ([]models.Release, error) {
	projection := `json_object('workflow',json_extract(spec,'$.workflow'),'runners',json_extract(spec,'$.runners'),'services',json((SELECT json_group_array(json_object('name',json_extract(value,'$.name'),'mode',json_extract(value,'$.mode'))) FROM json_each(spec,'$.services'))))`
	columns := strings.Replace(releaseColumns, "spec,", projection+",", 1)
	releases, err := s.queryReleases(`SELECT `+columns+` FROM releases WHERE `+selection+` ORDER BY started_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	byID := map[string]int{}
	for i := range releases {
		byID[releases[i].ID] = i
		releases[i].Workflow = string(releases[i].Spec.Workflow)
		releases[i].RunnerCount = len(releases[i].Spec.Runners)
	}
	rows, err := s.db.Query(`SELECT release_id,agent_id,agent_name,status,message,ended_at FROM release_targets WHERE release_id IN (SELECT id FROM releases WHERE `+selection+`) ORDER BY agent_name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var target models.ReleaseTarget
		var ended sql.NullTime
		if err = rows.Scan(&target.ReleaseID, &target.AgentID, &target.AgentName, &target.Status, &target.Message, &ended); err != nil {
			return nil, err
		}
		target.EndedAt = timePointer(ended)
		if i, ok := byID[target.ReleaseID]; ok {
			releases[i].Targets = append(releases[i].Targets, target)
		}
	}
	return releases, rows.Err()
}
