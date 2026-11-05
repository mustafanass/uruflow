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
	"github.com/mustafanass/uruflow/internal/ufp"
)

const terminalRequestTimeout = 10 * time.Second

type terminalBridge struct {
	agentID, sessionID string
	output             chan ufp.TerminalOutput
	done               <-chan struct{}
	disconnected       chan struct{}
	disconnectOnce     sync.Once
}

func (b *terminalBridge) AgentConnected(*models.Agent) {}
func (b *terminalBridge) AgentDisconnected(agentID string) {
	if agentID == b.agentID {
		b.disconnectOnce.Do(func() { close(b.disconnected) })
	}
}
func (b *terminalBridge) JobLog(string, ufp.JobLog)             {}
func (b *terminalBridge) JobStatus(string, ufp.JobStatus)       {}
func (b *terminalBridge) ContainerLog(string, ufp.ContainerLog) {}
func (b *terminalBridge) TerminalOutput(agentID string, output ufp.TerminalOutput) {
	if agentID != b.agentID || output.SessionID != b.sessionID {
		return
	}
	select {
	case b.output <- output:
	case <-b.done:
	}
}
func (e *Engine) OpenTerminal(ctx context.Context, agentID, containerID, sessionID string, columns, rows int, emit func(ufp.TerminalOutput) error) error {
	container, err := e.Container(agentID, containerID)
	if err != nil {
		return err
	}
	if container.Project == "" {
		return errors.New("only project containers provide a terminal")
	}
	done := make(chan struct{})
	bridge := &terminalBridge{agentID: agentID, sessionID: sessionID, output: make(chan ufp.TerminalOutput, 512), done: done, disconnected: make(chan struct{})}
	e.server.Link().Subscribe(bridge)
	opened := make(chan error, 1)
	go func() {
		requestCtx, cancel := context.WithTimeout(context.Background(), terminalRequestTimeout)
		defer cancel()
		_, err := e.server.Link().Request(requestCtx, agentID, ufp.MethodTerminalOpen, ufp.TerminalOpen{SessionID: sessionID, Project: container.Project, ContainerID: containerID, Columns: columns, Rows: rows})
		opened <- err
	}()
	handled := false
	closeTerminal := func() {
		closeCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_, _ = e.server.Link().Request(closeCtx, agentID, ufp.MethodTerminalClose, ufp.TerminalClose{SessionID: sessionID})
	}
	defer func() {
		close(done)
		e.server.Link().Unsubscribe(bridge)
		if handled {
			closeTerminal()
		} else {
			go func() { <-opened; closeTerminal() }()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-bridge.disconnected:
			return errors.New("the agent disconnected; open the terminal again after it returns")
		case err := <-opened:
			handled = true
			opened = nil
			if err != nil {
				return err
			}
		case output := <-bridge.output:
			if output.Complete {
				if output.Error != "" {
					return errors.New(output.Error)
				}
				return nil
			}
			if err := emit(output); err != nil {
				return err
			}
		}
	}
}
func (e *Engine) TerminalInput(agentID, sessionID string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), terminalRequestTimeout)
	defer cancel()
	_, err := e.server.Link().Request(ctx, agentID, ufp.MethodTerminalInput, ufp.TerminalInput{SessionID: sessionID, Data: data})
	return err
}
func (e *Engine) ResizeTerminal(agentID, sessionID string, columns, rows int) error {
	ctx, cancel := context.WithTimeout(context.Background(), terminalRequestTimeout)
	defer cancel()
	_, err := e.server.Link().Request(ctx, agentID, ufp.MethodTerminalResize, ufp.TerminalResize{SessionID: sessionID, Columns: columns, Rows: rows})
	return err
}
