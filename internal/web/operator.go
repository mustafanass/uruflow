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

package web

import (
	"context"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/observe"
	"github.com/mustafanass/uruflow/internal/ops"
	"github.com/mustafanass/uruflow/internal/ufp"
)

type Operator interface {
	Snapshot(context.Context) (observe.Snapshot, error)
	EnrollAgent(string, []string) (ops.Enrollment, error)
	RemoveAgent(string) error
	TrustRoot() ([]byte, error)
	CreateProject(string, string, string) (*models.Project, error)
	ReloadProjects() []string
	ProjectDocument(string, bool) (ops.Document, error)
	SaveProjectDocument(string, bool, string, string, func(models.Project) error) (ops.Document, error)
	Deploy(string, string, bool, bool) (*models.Release, error)
	StopProject(string) error
	ProjectReleases(string) ([]models.Release, error)
	Release(string) (*models.Release, error)
	Images(context.Context) (models.ImageCatalog, error)
	Image(context.Context, string, string) (models.Image, error)
	RefreshImages()
	RemoveImage(context.Context, string, string) error
	Alert(string) (*models.Alert, error)
	ResolveAlert(string) error
	RecentAlerts() ([]models.Alert, error)
	Container(string, string) (*models.Container, error)
	RestartContainer(string, string) error
	OpenTerminal(context.Context, string, string, string, int, int, func(ufp.TerminalOutput) error) error
	TerminalInput(string, string, []byte) error
	ResizeTerminal(string, string, int, int) error
	FollowReleaseLogs(context.Context, string, int64, func(models.LogLine) error) error
	FollowContainerLogs(context.Context, string, string, string, func(ufp.ContainerLog) error) error
}

var _ Operator = (*ops.Engine)(nil)
