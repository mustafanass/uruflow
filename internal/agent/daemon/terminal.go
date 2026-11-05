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
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const (
	terminalTimeout = 5 * time.Second
	terminalChunk   = 32 * 1024
)

type terminalSession struct {
	shell   *docker.Shell
	cancel  context.CancelFunc
	writeMu sync.Mutex
}

func (d *Daemon) handleTerminal(request *ufp.Request) error {
	switch request.Method {
	case ufp.MethodTerminalOpen:
		var payload ufp.TerminalOpen
		if err := request.Decode(&payload); err != nil {
			return err
		}
		if payload.SessionID == "" || payload.Project == "" || payload.ContainerID == "" {
			return fmt.Errorf("terminal request is incomplete")
		}
		if !ufp.ValidTerminalSize(payload.Columns, payload.Rows) {
			return fmt.Errorf("terminal size is invalid")
		}
		return d.openTerminal(payload)

	case ufp.MethodTerminalInput:
		var payload ufp.TerminalInput
		if err := request.Decode(&payload); err != nil {
			return err
		}
		if len(payload.Data) == 0 || len(payload.Data) > ufp.MaxTerminalInput {
			return fmt.Errorf("terminal input is invalid")
		}
		return d.writeTerminal(payload)

	case ufp.MethodTerminalResize:
		var payload ufp.TerminalResize
		if err := request.Decode(&payload); err != nil {
			return err
		}
		if !ufp.ValidTerminalSize(payload.Columns, payload.Rows) {
			return fmt.Errorf("terminal size is invalid")
		}
		return d.resizeTerminal(payload)

	case ufp.MethodTerminalClose:
		var payload ufp.TerminalClose
		if err := request.Decode(&payload); err != nil {
			return err
		}
		d.closeTerminal(payload.SessionID)
		return nil
	}

	return fmt.Errorf("unsupported method %q", request.Method)
}

func (d *Daemon) openTerminal(request ufp.TerminalOpen) error {
	d.connMu.RLock()
	parent := d.sessionCtx
	d.connMu.RUnlock()
	if parent == nil {
		return fmt.Errorf("agent session is not active")
	}

	ctx, cancel := context.WithTimeout(parent, terminalTimeout)
	defer cancel()
	_, owned, err := d.docker.ContainerOwnership(ctx, request.ContainerID, request.Project)
	if err != nil {
		return err
	}
	if !owned {
		return fmt.Errorf("container is not managed by project %s", request.Project)
	}
	shell, err := d.docker.OpenShell(ctx, request.ContainerID, request.Columns, request.Rows)
	if err != nil {
		return err
	}

	sessionCtx, stop := context.WithCancel(parent)
	session := &terminalSession{shell: shell, cancel: stop}
	d.terminalMu.Lock()
	if previous := d.terminals[request.SessionID]; previous != nil {
		previous.cancel()
	}
	d.terminals[request.SessionID] = session
	d.terminalMu.Unlock()

	go d.relayTerminal(sessionCtx, request.SessionID, session)
	logger.Info("[AGENT] terminal opened in %s for %s", shortID(request.ContainerID), request.Project)
	return nil
}

func (d *Daemon) relayTerminal(ctx context.Context, sessionID string, session *terminalSession) {
	go func() {
		<-ctx.Done()
		session.shell.Close()
	}()

	buffer := make([]byte, terminalChunk)
	var failure error
	for {
		count, err := session.shell.Read(buffer)
		if count > 0 {
			d.send(ufp.TopicTerminalOutput, ufp.TerminalOutput{SessionID: sessionID, Data: buffer[:count]})
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				failure = err
			}
			break
		}
	}

	closed := ctx.Err() != nil
	d.removeTerminal(sessionID, session)
	if closed {
		return
	}
	ended := ufp.TerminalOutput{SessionID: sessionID, Complete: true}
	if failure != nil {
		ended.Error = failure.Error()
	}
	d.send(ufp.TopicTerminalOutput, ended)
}

func (d *Daemon) writeTerminal(request ufp.TerminalInput) error {
	session := d.terminal(request.SessionID)
	if session == nil {
		return fmt.Errorf("terminal session has ended")
	}

	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	if err := session.shell.Write(request.Data, terminalTimeout); err != nil {
		failure := fmt.Errorf("terminal input failed: %w", err)
		session.cancel()
		d.send(ufp.TopicTerminalOutput, ufp.TerminalOutput{SessionID: request.SessionID, Complete: true, Error: failure.Error()})
		return failure
	}
	return nil
}

func (d *Daemon) resizeTerminal(request ufp.TerminalResize) error {
	session := d.terminal(request.SessionID)
	if session == nil {
		return fmt.Errorf("terminal session has ended")
	}

	ctx, cancel := context.WithTimeout(context.Background(), terminalTimeout)
	defer cancel()
	return d.docker.ResizeShell(ctx, session.shell, request.Columns, request.Rows)
}

func (d *Daemon) terminal(sessionID string) *terminalSession {
	d.terminalMu.Lock()
	defer d.terminalMu.Unlock()
	return d.terminals[sessionID]
}

func (d *Daemon) closeTerminal(sessionID string) {
	d.terminalMu.Lock()
	session := d.terminals[sessionID]
	delete(d.terminals, sessionID)
	d.terminalMu.Unlock()

	if session != nil {
		session.cancel()
	}
}

func (d *Daemon) removeTerminal(sessionID string, session *terminalSession) {
	d.terminalMu.Lock()
	if d.terminals[sessionID] == session {
		delete(d.terminals, sessionID)
	}
	d.terminalMu.Unlock()
	session.cancel()
}
