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

	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/models"
)

const followInterval = 500 * time.Millisecond

type Engine struct {
	server *api.Server
}

func New(server *api.Server) *Engine { return &Engine{server: server} }

func (e *Engine) Execute(ctx context.Context, args []string, input string, emit Emit) error {
	if emit == nil {
		return errors.New("event receiver is required")
	}
	command, err := grammar.Resolve(args)
	if err != nil {
		return err
	}
	if grammar.Path(command) == "help" {
		return emit(e.help())
	}
	switch args[0] {
	case "status":
		return e.status(ctx, emit)
	case "agent":
		return e.agents(args[1:], emit)
	case "project":
		return e.projects(ctx, args[1:], input, emit)
	case "release":
		return e.releases(ctx, args[1:], emit)
	case "container":
		return e.containers(ctx, args[1:], emit)
	case "events":
		return e.events(ctx, args[1:], emit)
	case "alert":
		return e.alerts(args[1:], emit)
	case "registry":
		return e.registry(ctx, args[1:], emit)
	case "secret":
		return e.secrets(args[1:], input, emit)
	default:
		return fmt.Errorf("command %q is handled by the workspace", grammar.Path(command))
	}
}

func (e *Engine) help() Event {
	commands := grammar.Visible()
	rows := make([][]string, 0, len(commands))
	for _, command := range commands {
		rows = append(rows, []string{grammar.Usage(command), command.Summary})
	}
	return Table("commands", []string{"COMMAND", "PURPOSE"}, rows)
}

func (e *Engine) status(ctx context.Context, emit Emit) error {
	store := e.server.Store()
	stats, err := store.Stats()
	if err != nil {
		return err
	}
	agents, err := store.ListAgents()
	if err != nil {
		return err
	}
	projectsList, err := store.ListProjects()
	if err != nil {
		return err
	}
	containers, err := store.ListContainers()
	if err != nil {
		return err
	}
	active, err := store.ListActiveReleases()
	if err != nil {
		return err
	}
	registryState := "healthy"
	healthCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := e.server.Registry().Health(healthCtx); err != nil {
		registryState = "unavailable"
	}
	if err := emit(Event{Type: EventResult, Time: time.Now(), Title: "fleet", Data: map[string]any{
		"agents_online": stats.AgentsOnline, "agents_total": stats.AgentsTotal,
		"projects": stats.ProjectsTotal, "containers_running": stats.ContainersRunning,
		"releases_active": len(active), "alerts": stats.AlertsActive, "registry": registryState,
	}}); err != nil {
		return err
	}
	counts := make(map[string]int)
	for _, container := range containers {
		counts[container.AgentID]++
	}
	agentRows := make([][]string, 0, len(agents))
	for _, agent := range agents {
		cpu, memory, disk := "–", "–", "–"
		if agent.Metrics != nil {
			cpu = fmt.Sprintf("%.0f%%", agent.Metrics.CPUPercent)
			memory = bytes(agent.Metrics.MemoryUsed) + "/" + bytes(agent.Metrics.MemoryTotal)
			disk = fmt.Sprintf("%.0f%%", agent.Metrics.DiskPercent)
		}
		agentRows = append(agentRows, []string{agent.Name, listRoles(agent.Roles), string(agent.Status),
			strconv.Itoa(counts[agent.ID]), cpu, memory, disk, since(agent.LastSeen)})
	}
	if err := emit(Table("agents", []string{"NAME", "ROLES", "STATE", "CTR", "CPU", "MEMORY", "DISK", "SEEN"}, agentRows)); err != nil {
		return err
	}
	releases, err := store.ListReleases(100)
	if err != nil {
		return err
	}
	latest := make(map[string]models.Release)
	for _, release := range releases {
		if _, found := latest[release.Project]; !found {
			latest[release.Project] = release
		}
	}
	projectRows := make([][]string, 0, len(projectsList))
	for _, project := range projectsList {
		release := latest[project.Name]
		state, revision, age := "idle", "–", "–"
		if release.ID != "" {
			state, revision, age = string(release.Status), short(release.Commit, 8), since(release.StartedAt)
		}
		projectRows = append(projectRows, []string{project.Name, project.EffectiveWorkflow(), state, revision, age})
	}
	return emit(Table("projects", []string{"NAME", "WORKFLOW", "STATE", "REVISION", "AGE"}, projectRows))
}

func listRoles(roles []models.Role) string {
	values := make([]string, len(roles))
	for index, role := range roles {
		values[index] = string(role)
	}
	return list(values)
}
