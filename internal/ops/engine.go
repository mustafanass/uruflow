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
	if len(args) == 0 || args[0] == "help" {
		return emit(e.help())
	}
	switch args[0] {
	case "status":
		return e.status(ctx, emit)
	case "agent", "agents":
		return e.agents(args[1:], emit)
	case "project", "projects":
		return e.projects(ctx, args[1:], input, emit)
	case "release", "releases":
		return e.releases(ctx, args[1:], emit)
	case "container", "containers":
		return e.containers(ctx, args[1:], emit)
	case "events":
		return e.events(ctx, emit)
	case "alert", "alerts":
		return e.alerts(args[1:], emit)
	case "registry":
		return e.registry(ctx, args[1:], emit)
	case "secret", "secrets":
		return e.secrets(args[1:], input, emit)
	default:
		return fmt.Errorf("unknown command %q; run help", args[0])
	}
}

func (e *Engine) help() Event {
	return Table("commands", []string{"COMMAND", "PURPOSE"}, [][]string{
		{"status", "fleet health and active work"},
		{"events", "follow new fleet activity"},
		{"deploy NAME", "short form of project deploy"},
		{"rollback NAME", "short form of project rollback"},
		{"logs ID [--follow]", "short form of release logs"},
		{"agent list", "list enrolled agents"},
		{"agent show NAME", "inspect resources and containers"},
		{"agent add NAME [--roles …]", "enrol and show one-time credentials"},
		{"agent remove NAME", "remove an enrolled agent"},
		{"project list", "list YAML-owned projects"},
		{"project show NAME", "inspect a project and its services"},
		{"project edit NAME", "open authoritative environment YAML"},
		{"project validate FILE", "validate environment YAML"},
		{"project apply PROJECT ENV FILE", "validate, save and reload YAML"},
		{"project reload", "reload authoritative YAML files"},
		{"project deploy NAME [--no-follow]", "start and follow a release"},
		{"project rollback NAME [--no-follow]", "roll back and follow"},
		{"project stop NAME", "stop project containers"},
		{"release list [--limit N]", "list recent releases"},
		{"release show ID", "inspect release state and targets"},
		{"release logs ID [--follow]", "read or follow release output"},
		{"release follow ID", "attach to a release"},
		{"container list", "list managed containers"},
		{"container logs AGENT ID [--follow]", "stream application output"},
		{"registry list", "registry catalog"},
		{"registry remove REPOSITORY TAG", "delete an image manifest"},
		{"alert list", "list active operational alerts"},
		{"alert resolve ID", "resolve an alert"},
		{"secret list", "list encrypted secret names"},
		{"secret set NAME", "store a value using masked input"},
		{"secret remove NAME", "remove an encrypted secret"},
		{"clear", "clear the workspace transcript"},
		{"exit", "close the workspace"},
	})
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
