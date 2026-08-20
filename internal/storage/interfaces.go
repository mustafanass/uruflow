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

package storage

import (
	"errors"

	"github.com/mustafanass/uruflow/internal/models"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	CreateAgent(agent *models.Agent) error
	UpdateAgent(agent *models.Agent) error
	SetAgentStatus(id string, status models.AgentStatus) error
	SetAgentMetrics(id string, metrics *models.Metrics) error
	GetAgent(id string) (*models.Agent, error)
	GetAgentByName(name string) (*models.Agent, error)
	ListAgents() ([]models.Agent, error)
	DeleteAgent(id string) error

	SaveProject(project *models.Project) error
	GetProject(name string) (*models.Project, error)
	ListProjects() ([]models.Project, error)
	DeleteProject(name string) error

	CreateRelease(release *models.Release) error
	UpdateRelease(release *models.Release) error
	GetRelease(id string) (*models.Release, error)
	ListReleases(limit int) ([]models.Release, error)
	ListReleasesByProject(project string, limit int) ([]models.Release, error)
	LastSuccessfulRelease(project string) (*models.Release, error)

	SaveReleaseTarget(target *models.ReleaseTarget) error
	ListReleaseTargets(releaseID string) ([]models.ReleaseTarget, error)

	AppendLog(line *models.LogLine) error
	ListLogs(releaseID string) ([]models.LogLine, error)

	UpsertContainer(container *models.Container) error
	ListContainers() ([]models.Container, error)
	ListContainersByAgent(agentID string) ([]models.Container, error)
	ReplaceContainers(agentID string, containers []models.Container) error

	CreateAlert(alert *models.Alert) error
	ResolveAlert(id string) error
	ListActiveAlerts() ([]models.Alert, error)
	ListRecentAlerts(limit int) ([]models.Alert, error)

	SetSecret(name string, value []byte) error
	GetSecret(name string) ([]byte, error)
	ListSecrets() ([]models.Secret, error)
	DeleteSecret(name string) error

	SetAllAgentsOffline() error
	FailUnfinishedReleases(message string) error

	Stats() (*Stats, error)
	Close() error
}

type Stats struct {
	AgentsTotal       int
	AgentsOnline      int
	BuildersOnline    int
	RunnersOnline     int
	ProjectsTotal     int
	ReleasesTotal     int
	ReleasesToday     int
	SuccessRate       float64
	ContainersRunning int
	ContainersStopped int
	AlertsActive      int
}
