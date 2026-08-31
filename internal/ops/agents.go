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
	"strconv"
	"time"

	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/roles"
	"github.com/mustafanass/uruflow/internal/services"
	"github.com/mustafanass/uruflow/pkg/helper"
)

func (e *Engine) agents(args []string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		agents, err := e.server.Store().ListAgents()
		if err != nil {
			return err
		}
		containers, err := e.server.Store().ListContainers()
		if err != nil {
			return err
		}
		counts := make(map[string]int)
		for _, container := range containers {
			counts[container.AgentID]++
		}
		rows := make([][]string, 0, len(agents))
		for _, agent := range agents {
			cpu, memory, disk := "–", "–", "–"
			if agent.Metrics != nil {
				cpu = fmt.Sprintf("%.0f%%", agent.Metrics.CPUPercent)
				memory = fmt.Sprintf("%.0f%%", agent.Metrics.MemoryPercent)
				disk = fmt.Sprintf("%.0f%%", agent.Metrics.DiskPercent)
			}
			rows = append(rows, []string{agent.Name, listRoles(agent.Roles), string(agent.Status),
				strconv.Itoa(counts[agent.ID]), cpu, memory, disk, agent.Version, since(agent.LastSeen)})
		}
		return emit(Table("agents", []string{"NAME", "ROLES", "STATE", "CTR", "CPU", "MEM", "DISK", "VERSION", "SEEN"}, rows))
	}
	if args[0] == "add" {
		if len(args) < 2 {
			return grammar.UsageError("agent", "add")
		}
		parsedRoles, roleNames, err := parseAgentRoles(args[2:])
		if err != nil {
			return err
		}
		agent := &models.Agent{ID: helper.GenerateID(), Name: args[1], Key: helper.GenerateToken(), Roles: parsedRoles}
		if !models.ValidResourceName(agent.Name) {
			return fmt.Errorf("invalid agent name %q", agent.Name)
		}
		if err := e.server.Store().CreateAgent(agent); err != nil {
			return fmt.Errorf("an agent named %s already exists", agent.Name)
		}
		cfg := e.server.Config()
		return emit(Event{Type: EventResult, Time: time.Now(), Title: "agent enrolled", Data: map[string]any{
			"name": agent.Name, "id": agent.ID, "key": agent.Key, "roles": roleNames,
			"server":         fmt.Sprintf("%s:%d", cfg.Server.Advertise, cfg.Server.UFPPort),
			"ca_certificate": cfg.CACertPath(),
		}})
	}
	if args[0] == "remove" {
		if len(args) != 2 {
			return grammar.UsageError("agent", "remove")
		}
		agent, err := e.server.Store().GetAgentByName(args[1])
		if err != nil {
			return fmt.Errorf("agent %s: %w", args[1], err)
		}
		if err := services.DeleteAgent(e.server.Store(), e.server.Link().Revoke, agent.ID); err != nil {
			return err
		}
		return emit(Message("success", "removed agent "+agent.Name))
	}
	if args[0] != "show" || len(args) != 2 {
		return grammar.GroupUsageError("agent")
	}
	agent, err := e.server.Store().GetAgentByName(args[1])
	if err != nil {
		return fmt.Errorf("agent %s: %w", args[1], err)
	}
	metrics := map[string]any{}
	if agent.Metrics != nil {
		metrics = map[string]any{
			"cpu_percent":  agent.Metrics.CPUPercent,
			"memory_used":  agent.Metrics.MemoryUsed,
			"memory_total": agent.Metrics.MemoryTotal,
			"disk_used":    agent.Metrics.DiskUsed,
			"disk_total":   agent.Metrics.DiskTotal,
			"load_average": agent.Metrics.LoadAvg,
			"uptime":       agent.Metrics.Uptime,
		}
	}
	if err := emit(Event{Type: EventResult, Title: agent.Name, Time: time.Now(), Data: map[string]any{
		"id": agent.ID, "roles": agent.Roles, "state": agent.Status, "host": agent.Host,
		"hostname": agent.Hostname, "platform": agent.Platform, "version": agent.Version,
		"last_seen": agent.LastSeen, "metrics": metrics,
	}}); err != nil {
		return err
	}
	containers, err := e.server.Store().ListContainersByAgent(agent.ID)
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(containers))
	for _, container := range containers {
		rows = append(rows, []string{container.Name, containerKind(container), container.Project, container.Service, short(container.ID, 12),
			short(container.Image, 32), container.State, container.Health, fmt.Sprintf("%.0f%%", container.CPUPercent), bytes(container.MemoryUsage)})
	}
	return emit(Table("containers", []string{"NAME", "TYPE", "PROJECT", "SERVICE", "ID", "IMAGE", "STATE", "HEALTH", "CPU", "MEMORY"}, rows))
}

func parseAgentRoles(options []string) ([]models.Role, string, error) {
	value := string(models.RoleRunner)
	if len(options) > 0 {
		if len(options) != 2 || options[0] != "--roles" {
			return nil, "", grammar.UsageError("agent", "add")
		}
		value = options[1]
	}
	parsed, err := roles.Parse(value)
	if err != nil {
		return nil, "", err
	}
	return parsed, listRoles(parsed), nil
}
