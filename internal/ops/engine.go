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
	"sync"
	"time"

	"github.com/mustafanass/uruflow/internal/agent/metrics"
	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/models"
)

type Engine struct {
	server          *api.Server
	configurationMu sync.Mutex
	observationMu   sync.Mutex
	hostCollector   *metrics.Collector
	hostCache       *models.Metrics
	registryState   string
	registryChecked time.Time
	releaseMu       sync.Mutex
	releaseRevision int64
	releaseCache    []models.Release
}

func New(server *api.Server) *Engine {
	return &Engine{server: server, hostCollector: metrics.NewCollector()}
}
