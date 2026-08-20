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

package daemon

import (
	"context"
	"time"

	"github.com/urustack/uruflow/internal/ufp"
	"github.com/urustack/uruflow/pkg/logger"
)

func (d *Daemon) followContainer(request ufp.LogsFollow) {
	d.stopFollow(request.ContainerID)

	ctx, cancel := context.WithCancel(context.Background())
	d.streamMu.Lock()
	d.streams[request.ContainerID] = cancel
	d.streamMu.Unlock()

	go func() {
		defer d.stopFollow(request.ContainerID)

		logger.Debug("[AGENT] following logs for %s", shortID(request.ContainerID))
		err := d.docker.StreamLogs(ctx, request.ContainerID, request.Tail, true,
			func(stream, line string) {
				d.send(ufp.TopicContainerLog, ufp.ContainerLog{
					ContainerID: request.ContainerID,
					Stream:      stream,
					Line:        line,
					Timestamp:   time.Now().Unix(),
				})
			})
		if err != nil && ctx.Err() == nil {
			logger.Debug("[AGENT] log stream for %s ended: %v", shortID(request.ContainerID), err)
		}
	}()
}

func (d *Daemon) stopFollow(containerID string) {
	d.streamMu.Lock()
	defer d.streamMu.Unlock()

	if cancel, found := d.streams[containerID]; found {
		cancel()
		delete(d.streams, containerID)
	}
}

func (d *Daemon) cancelStreams() {
	d.streamMu.Lock()
	defer d.streamMu.Unlock()

	for id, cancel := range d.streams {
		cancel()
		delete(d.streams, id)
	}
}
