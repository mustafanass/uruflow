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

const projectColumns = `name, git_url, branch, dockerfile, context, build_args,
	builder, runners, auto_deploy, workflow, runtime, services, env, resources, source, created_at`

func (s *Store) SaveProject(project *models.Project) error {
	_, err := s.db.Exec(`
		INSERT INTO projects (name, git_url, branch, dockerfile, context, build_args,
		                      builder, runners, auto_deploy, workflow, runtime, services, env, resources, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			git_url = excluded.git_url,
			branch = excluded.branch,
			dockerfile = excluded.dockerfile,
			context = excluded.context,
			build_args = excluded.build_args,
			builder = excluded.builder,
			runners = excluded.runners,
			auto_deploy = excluded.auto_deploy,
			workflow = excluded.workflow,
			runtime = excluded.runtime,
			services = excluded.services,
			env = excluded.env,
			resources = excluded.resources,
			source = excluded.source`,
		project.Name, project.GitURL, project.Branch, project.Dockerfile, project.Context,
		encodeJSON(project.BuildArgs), project.Builder, encodeJSON(project.Runners),
		project.AutoDeploy, project.Workflow, encodeJSON(project.Runtime), encodeJSON(project.Services),
		project.Env, encodeJSON(models.ProjectResources{Networks: project.Networks, Volumes: project.Volumes}), project.Source)
	return err
}

func (s *Store) GetProject(name string) (*models.Project, error) {
	return scanProject(s.db.QueryRow(`SELECT `+projectColumns+` FROM projects WHERE name = ?`, name))
}

func (s *Store) ListProjects() ([]models.Project, error) {
	rows, err := s.db.Query(`SELECT ` + projectColumns + ` FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]models.Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *project)
	}
	return projects, rows.Err()
}

func (s *Store) DeleteProject(name string) error {
	result, err := s.db.Exec(`DELETE FROM projects WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func scanProject(row scanner) (*models.Project, error) {
	var project models.Project
	var buildArgs, runners, runtime, services, resources string

	err := row.Scan(&project.Name, &project.GitURL, &project.Branch, &project.Dockerfile,
		&project.Context, &buildArgs, &project.Builder, &runners,
		&project.AutoDeploy, &project.Workflow, &runtime, &services, &project.Env, &resources, &project.Source, &project.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	decodeJSON(buildArgs, &project.BuildArgs)
	decodeJSON(runners, &project.Runners)
	decodeJSON(runtime, &project.Runtime)
	decodeJSON(services, &project.Services)
	var projectResources models.ProjectResources
	decodeJSON(resources, &projectResources)
	project.Networks = projectResources.Networks
	project.Volumes = projectResources.Volumes
	return &project, nil
}
