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
	"fmt"
	"strconv"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

type containerBridge struct {
	agentID   string
	container string
	entries   chan ufp.ContainerLog
}

func (b *containerBridge) AgentConnected(*models.Agent)    {}
func (b *containerBridge) AgentDisconnected(string)        {}
func (b *containerBridge) JobLog(string, ufp.JobLog)       {}
func (b *containerBridge) JobStatus(string, ufp.JobStatus) {}
func (b *containerBridge) ContainerLog(agentID string, entry ufp.ContainerLog) {
	if agentID != b.agentID || entry.ContainerID != b.container {
		return
	}
	select {
	case b.entries <- entry:
	default:
	}
}

func (e *Engine) containers(ctx context.Context, args []string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		values, err := e.server.Store().ListContainers()
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, container := range values {
			agent := container.AgentID
			if value, agentErr := e.server.Store().GetAgent(container.AgentID); agentErr == nil {
				agent = value.Name
			}
			rows = append(rows, []string{agent, container.Project, container.Service, short(container.ID, 12),
				container.State, container.Health, fmt.Sprintf("%.0f%%", container.CPUPercent), bytes(container.MemoryUsage)})
		}
		return emit(Table("containers", []string{"AGENT", "PROJECT", "SERVICE", "ID", "STATE", "HEALTH", "CPU", "MEMORY"}, rows))
	}
	if args[0] != "logs" || len(args) < 3 {
		return errors.New("usage: container list | container logs AGENT CONTAINER [--tail N] [--follow]")
	}
	agent, err := e.server.Store().GetAgentByName(args[1])
	if err != nil {
		return fmt.Errorf("agent %s: %w", args[1], err)
	}
	tail, follow := 200, false
	for index := 3; index < len(args); index++ {
		switch args[index] {
		case "--follow":
			follow = true
		case "--tail":
			if index+1 >= len(args) {
				return errors.New("--tail requires a line count")
			}
			parsed, parseErr := strconv.Atoi(args[index+1])
			if parseErr != nil || parsed < 0 || parsed > 10000 {
				return errors.New("--tail must be between 0 and 10000")
			}
			tail, index = parsed, index+1
		}
	}
	bridge := &containerBridge{agentID: agent.ID, container: args[2], entries: make(chan ufp.ContainerLog, 512)}
	e.server.Link().Subscribe(bridge)
	defer e.server.Link().Unsubscribe(bridge)
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	_, err = e.server.Link().Request(requestCtx, agent.ID, ufp.MethodLogsFollow, ufp.LogsFollow{ContainerID: args[2], Tail: tail})
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		stopCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_, _ = e.server.Link().Request(stopCtx, agent.ID, ufp.MethodLogsStop, ufp.LogsStop{ContainerID: args[2]})
	}()
	quiet := time.NewTimer(750 * time.Millisecond)
	defer quiet.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-bridge.entries:
			if !quiet.Stop() {
				select {
				case <-quiet.C:
				default:
				}
			}
			quiet.Reset(750 * time.Millisecond)
			level := "info"
			if entry.Stream == ufp.StreamStderr {
				level = "warning"
			}
			if err := emit(Event{Type: EventLog, Time: time.Unix(entry.Timestamp, 0), Level: level,
				Title: agent.Name, Message: entry.Line, Operation: args[2]}); err != nil {
				return err
			}
		case <-quiet.C:
			if !follow {
				return nil
			}
			quiet.Reset(750 * time.Millisecond)
		}
	}
}
