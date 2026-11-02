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
	"errors"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"
)

type logStream struct{ cancel context.CancelFunc }

func (d *Daemon) followContainer(request ufp.LogsFollow) {
	key := request.StreamID
	if key == "" {
		key = request.ContainerID
	}
	ctx, cancel := context.WithCancel(context.Background())
	stream := &logStream{cancel}
	d.streamMu.Lock()
	if previous := d.streams[key]; previous != nil {
		previous.cancel()
	}
	d.streams[key] = stream
	d.streamMu.Unlock()
	go func() {
		defer func() {
			cancel()
			d.streamMu.Lock()
			if d.streams[key] == stream {
				delete(d.streams, key)
			}
			d.streamMu.Unlock()
		}()
		after, _ := ufp.ParseLogPosition(request.After)
		start := after.Since()
		var err error
		if start.IsZero() {
			readCtx, stop := context.WithTimeout(ctx, 15*time.Second)
			start, err = d.docker.LogStart(readCtx, request.ContainerID, request.Tail)
			stop()
		}
		position := ufp.LogPosition{Start: start}
		checkpoint := after
		if checkpoint.Start.IsZero() {
			checkpoint.Start = start
		}
		if err == nil {
			err = d.docker.StreamLogsSince(ctx, request.ContainerID, request.Tail, start, true, func(kind, line string, at time.Time) {
				position.Advance(kind, at)
				if after.Includes(kind, at, position.Offset(kind)) {
					return
				}
				d.send(ufp.TopicContainerLog, ufp.ContainerLog{Cursor: checkpoint.Merge(position), StreamID: request.StreamID, ContainerID: request.ContainerID, Stream: kind, Line: line, Timestamp: at.Unix()})
			})
		}
		if err != nil && ctx.Err() == nil {
			logger.Debug("[AGENT] log stream for %s ended: %v", shortID(request.ContainerID), err)
		}
		if ctx.Err() == nil {
			ended := ufp.ContainerLog{StreamID: request.StreamID, ContainerID: request.ContainerID, Complete: true, Timestamp: time.Now().Unix()}
			if err != nil {
				ended.Error = err.Error()
				if errors.Is(err, docker.ErrNotFound) {
					ended.ErrorCode = 404
				}
			}
			d.send(ufp.TopicContainerLog, ended)
		}
	}()
}
func (d *Daemon) stopFollow(key string) {
	d.streamMu.Lock()
	defer d.streamMu.Unlock()
	if stream := d.streams[key]; stream != nil {
		stream.cancel()
		delete(d.streams, key)
	}
}
func (d *Daemon) cancelStreams() {
	d.streamMu.Lock()
	defer d.streamMu.Unlock()
	for key, stream := range d.streams {
		stream.cancel()
		delete(d.streams, key)
	}
}
