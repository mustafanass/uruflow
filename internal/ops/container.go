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
	"context"
	"errors"
	"sync"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/helper"
)

type containerBridge struct {
	agentID, container, streamID string
	entries                      chan ufp.ContainerLog
	done                         <-chan struct{}
	disconnected                 chan struct{}
	disconnectOnce               sync.Once
}

func (b *containerBridge) AgentConnected(*models.Agent) {}
func (b *containerBridge) AgentDisconnected(agentID string) {
	if agentID == b.agentID && b.disconnected != nil {
		b.disconnectOnce.Do(func() { close(b.disconnected) })
	}
}
func (b *containerBridge) JobLog(string, ufp.JobLog)       {}
func (b *containerBridge) JobStatus(string, ufp.JobStatus) {}
func (b *containerBridge) ContainerLog(agentID string, entry ufp.ContainerLog) {
	if agentID != b.agentID || entry.ContainerID != b.container || entry.StreamID != b.streamID {
		return
	}
	select {
	case b.entries <- entry:
	case <-b.done:
	}
}
func (e *Engine) Container(agentID, containerID string) (*models.Container, error) {
	rows, err := e.server.Store().ListContainersByAgent(agentID)
	if err != nil {
		return nil, err
	}
	for _, c := range rows {
		if c.ID == containerID {
			return &c, nil
		}
	}
	return nil, storage.ErrNotFound
}
func (e *Engine) FollowContainerLogs(ctx context.Context, agentID, containerID, after string, emit func(ufp.ContainerLog) error) error {
	if _, err := ufp.ParseLogPosition(after); err != nil {
		return err
	}
	rows, err := e.server.Store().ListContainersByAgent(agentID)
	if err != nil {
		return err
	}
	found := false
	for _, c := range rows {
		if c.ID == containerID {
			found = true
			break
		}
	}
	if !found {
		return storage.ErrNotFound
	}
	done := make(chan struct{})
	bridge := &containerBridge{agentID: agentID, container: containerID, streamID: helper.GenerateID(), entries: make(chan ufp.ContainerLog, 512), done: done, disconnected: make(chan struct{})}
	e.server.Link().Subscribe(bridge)
	responseDone := make(chan error, 1)
	go func() {

		requestCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		response, err := e.server.Link().Request(requestCtx, agentID, ufp.MethodLogsFollow, ufp.LogsFollow{ContainerID: containerID, StreamID: bridge.streamID, Tail: 200, After: after})
		if err == nil {
			var accepted ufp.Accepted
			err = response.Decode(&accepted)
			if err == nil && accepted.StreamID != bridge.streamID {
				err = errors.New("update this agent to support independent browser log viewers")
			}
		}
		responseDone <- err
	}()
	handled := false
	stopStream := func() {
		stopCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_, _ = e.server.Link().Request(stopCtx, agentID, ufp.MethodLogsStop, ufp.LogsStop{ContainerID: containerID, StreamID: bridge.streamID})
	}
	defer func() {
		close(done)
		e.server.Link().Unsubscribe(bridge)
		if handled {
			stopStream()
		} else {
			go func() { <-responseDone; stopStream() }()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-bridge.disconnected:
			return errors.New("the agent disconnected; reconnect after it returns")
		case err := <-responseDone:
			handled = true
			responseDone = nil
			if err != nil {
				return err
			}
		case entry := <-bridge.entries:
			if entry.Complete {
				if entry.Error != "" {
					return &LogStreamError{Code: entry.ErrorCode, Message: entry.Error}
				}
				return nil
			}
			if entry.Cursor == "" {
				return errors.New("update this agent to support resumable container logs")
			}
			if _, err := ufp.ParseLogPosition(entry.Cursor); err != nil {
				return err
			}
			if err := emit(entry); err != nil {
				return err
			}
		}
	}
}

type LogStreamError struct {
	Code    int
	Message string
}

func (e *LogStreamError) Error() string { return e.Message }
