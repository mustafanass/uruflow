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

package ops

import (
	"fmt"
	"slices"

	"github.com/mustafanass/uruflow/internal/models"
)

type PushSource struct {
	Repository string   `json:"repository"`
	Branch     string   `json:"branch"`
	Services   []string `json:"services"`
	Enabled    bool     `json:"enabled"`
}

type PushSettings struct {
	Receiving bool         `json:"receiving"`
	Sources   []PushSource `json:"sources"`
}

func (e *Engine) PushSettings(name string) (PushSettings, error) {
	project, err := e.server.Store().GetProject(name)
	if err != nil {
		return PushSettings{}, err
	}
	stored, err := e.server.Store().ListPushSources()
	if err != nil {
		return PushSettings{}, err
	}
	enabled := make(map[models.PushSource]bool, len(stored))
	for _, source := range stored {
		enabled[source] = true
	}
	settings := PushSettings{Receiving: e.server.Webhooks().Enabled(), Sources: []PushSource{}}
	positions := make(map[models.PushSource]int)
	for _, service := range project.ServiceList() {
		if !service.Built() {
			continue
		}
		key := models.PushSource{Project: project.Name, Repository: service.GitURL, Branch: service.Branch}
		position, seen := positions[key]
		if !seen {
			position = len(settings.Sources)
			positions[key] = position
			settings.Sources = append(settings.Sources, PushSource{Repository: key.Repository, Branch: key.Branch, Enabled: enabled[key]})
		}
		settings.Sources[position].Services = append(settings.Sources[position].Services, service.Name)
	}
	return settings, nil
}

func (e *Engine) SetPushSource(name, repository, branch string, enabled bool) (PushSettings, error) {
	settings, err := e.PushSettings(name)
	if err != nil {
		return PushSettings{}, err
	}
	known := slices.ContainsFunc(settings.Sources, func(source PushSource) bool {
		return source.Repository == repository && source.Branch == branch
	})
	if enabled && !known {
		return PushSettings{}, fmt.Errorf("project %s builds nothing from %s on %s", name, repository, branch)
	}
	source := models.PushSource{Project: name, Repository: repository, Branch: branch}
	if err := e.server.Store().SetPushSource(source, enabled); err != nil {
		return PushSettings{}, err
	}
	return e.PushSettings(name)
}
