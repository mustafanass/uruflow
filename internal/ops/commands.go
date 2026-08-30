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
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/projects"
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
			return errors.New("usage: agent add NAME [--roles builder|runner|builder,runner]")
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
			return errors.New("usage: agent remove NAME")
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
		return errors.New("usage: agent list | agent show NAME | agent add NAME | agent remove NAME")
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
		rows = append(rows, []string{container.Project, container.Service, short(container.ID, 12),
			short(container.Image, 32), container.State, container.Health, fmt.Sprintf("%.0f%%", container.CPUPercent), bytes(container.MemoryUsage)})
	}
	return emit(Table("containers", []string{"PROJECT", "SERVICE", "ID", "IMAGE", "STATE", "HEALTH", "CPU", "MEMORY"}, rows))
}

func parseAgentRoles(options []string) ([]models.Role, string, error) {
	value := string(models.RoleRunner)
	if len(options) > 0 {
		if len(options) != 2 || options[0] != "--roles" {
			return nil, "", errors.New("usage: agent add NAME [--roles builder|runner|builder,runner]")
		}
		value = options[1]
	}
	parsed, err := roles.Parse(value)
	if err != nil {
		return nil, "", err
	}
	return parsed, listRoles(parsed), nil
}

func (e *Engine) projects(ctx context.Context, args []string, input string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		values, err := e.server.Store().ListProjects()
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, project := range values {
			rows = append(rows, []string{project.Name, project.Env, project.EffectiveWorkflow(),
				strconv.Itoa(len(project.ServiceList())), project.Branch, project.Source})
		}
		return emit(Table("projects", []string{"NAME", "ENV", "WORKFLOW", "SERVICES", "BRANCH", "SOURCE"}, rows))
	}
	switch args[0] {
	case "path":
		if len(args) != 2 {
			return errors.New("usage: project path NAME")
		}
		project, err := e.server.Store().GetProject(args[1])
		if err != nil {
			return fmt.Errorf("project %s: %w", args[1], err)
		}
		if project.Source == "" {
			return fmt.Errorf("project %s is not owned by a YAML file", project.Name)
		}
		return emit(Event{Type: EventResult, Time: time.Now(), Title: "project file", Message: project.Source,
			Data: map[string]any{"name": project.Name, "path": project.Source}})
	case "show":
		if len(args) != 2 {
			return errors.New("usage: project show NAME")
		}
		project, err := e.server.Store().GetProject(args[1])
		if err != nil {
			return fmt.Errorf("project %s: %w", args[1], err)
		}
		builder := project.Builder
		if agent, agentErr := e.server.Store().GetAgent(project.Builder); agentErr == nil {
			builder = agent.Name
		}
		runners := make([]string, 0, len(project.Runners))
		for _, id := range project.Runners {
			if agent, agentErr := e.server.Store().GetAgent(id); agentErr == nil {
				runners = append(runners, agent.Name)
			} else {
				runners = append(runners, id)
			}
		}
		if err := emit(Table(project.Name, []string{"FIELD", "VALUE"}, [][]string{
			{"source", project.Source}, {"branch", project.Branch}, {"workflow", project.EffectiveWorkflow()},
			{"builder", builder}, {"runners", strings.Join(runners, ", ")},
			{"auto deploy", strconv.FormatBool(project.AutoDeploy)}, {"services", strconv.Itoa(len(project.ServiceList()))},
		})); err != nil {
			return err
		}
		rows := make([][]string, 0, len(project.ServiceList()))
		for _, service := range project.ServiceList() {
			source := service.Image
			if service.Built() {
				source = "build " + service.BuildFile()
			}
			rows = append(rows, []string{service.Name, service.EffectiveMode(), source, strings.Join(models.FormatPorts(service.Ports), ","), service.Restart})
		}
		return emit(Table("services", []string{"NAME", "MODE", "SOURCE", "PORTS", "RESTART"}, rows))
	case "reload":
		loaded := e.server.ReloadProjects()
		problems := e.server.ProjectProblems()
		if err := emit(Message("success", fmt.Sprintf("loaded %d project environments from YAML", loaded))); err != nil {
			return err
		}
		for _, problem := range problems {
			if err := emit(Message("warning", fmt.Sprintf("%s: %v", problem.Path, problem.Reason))); err != nil {
				return err
			}
		}
		return nil
	case "deploy", "rollback":
		if len(args) < 2 {
			return fmt.Errorf("usage: project %s NAME [--no-follow]", args[0])
		}
		var release *models.Release
		var err error
		if args[0] == "deploy" {
			release, err = e.server.Pipeline().Trigger(args[1], "", models.TriggerManual)
		} else {
			release, err = e.server.Pipeline().Rollback(args[1], "")
		}
		if err != nil {
			return err
		}
		if err := emit(Event{Type: EventMessage, Time: time.Now(), Level: "success", Operation: release.ID,
			Message: fmt.Sprintf("release %s started for %s", release.ID, release.Project)}); err != nil {
			return err
		}
		if contains(args[2:], "--no-follow") {
			return nil
		}
		return e.followRelease(ctx, release.ID, emit)
	case "stop":
		if len(args) != 2 {
			return errors.New("usage: project stop NAME")
		}
		if err := e.server.Pipeline().Stop(args[1]); err != nil {
			return err
		}
		return emit(Message("success", args[1]+" stopped on every runner"))
	case "validate":
		if len(args) != 2 {
			return errors.New("usage: project validate FILE")
		}
		content, err := readContent(args[1], input)
		if err != nil {
			return err
		}
		if err := projects.ValidateEnvironmentYAML(content); err != nil {
			return err
		}
		return emit(Message("success", "YAML is valid"))
	case "apply":
		if len(args) != 4 {
			return errors.New("usage: project apply PROJECT ENV FILE")
		}
		return e.applyProject(args[1], args[2], args[3], input, emit)
	default:
		return fmt.Errorf("unknown project command %q", args[0])
	}
}

func readContent(path, input string) (string, error) {
	if path == "-" {
		if strings.TrimSpace(input) == "" {
			return "", errors.New("no YAML was supplied on stdin")
		}
		return input, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (e *Engine) applyProject(project, environment, source, input string, emit Emit) error {
	if !models.ValidResourceName(project) || !models.ValidResourceName(environment) {
		return errors.New("project and environment must be lowercase resource names")
	}
	content, err := readContent(source, input)
	if err != nil {
		return err
	}
	if err := projects.ValidateEnvironmentYAML(content); err != nil {
		return fmt.Errorf("validate YAML: %w", err)
	}
	_, destination, _ := e.server.Loader().Paths(project, environment)
	if _, err := os.Stat(filepath.Dir(destination)); err != nil {
		return fmt.Errorf("project definition does not exist: %s", filepath.Join(filepath.Dir(destination), projects.ProjectFile))
	}
	previous, previousErr := os.ReadFile(destination)
	hadPrevious := previousErr == nil
	if previousErr != nil && !errors.Is(previousErr, os.ErrNotExist) {
		return previousErr
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".uruflow-*.yaml")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.WriteString(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	loaded := e.server.ReloadProjects()
	for _, problem := range e.server.ProjectProblems() {
		if problem.Path == destination {
			if hadPrevious {
				_ = os.WriteFile(destination, previous, 0o644)
			} else {
				_ = os.Remove(destination)
			}
			e.server.ReloadProjects()
			return fmt.Errorf("YAML was not applied: %w", problem.Reason)
		}
	}
	return emit(Message("success", fmt.Sprintf("saved %s and loaded %d project environments", destination, loaded)))
}

func (e *Engine) releases(ctx context.Context, args []string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		limit := 25
		if len(args) == 3 && args[1] == "--limit" {
			parsed, err := strconv.Atoi(args[2])
			if err != nil || parsed < 1 || parsed > 1000 {
				return errors.New("--limit must be between 1 and 1000")
			}
			limit = parsed
		}
		values, err := e.server.Store().ListReleases(limit)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, release := range values {
			rows = append(rows, []string{release.ID, release.Project, string(release.Status),
				string(release.Trigger), short(release.Commit, 8), duration(release.Duration), since(release.StartedAt)})
		}
		return emit(Table("releases", []string{"ID", "PROJECT", "STATE", "TRIGGER", "REVISION", "DURATION", "AGE"}, rows))
	}
	if len(args) < 2 {
		return errors.New("usage: release show ID | release logs ID [--follow] | release follow ID")
	}
	switch args[0] {
	case "show":
		release, err := e.server.Store().GetRelease(args[1])
		if err != nil {
			return err
		}
		if err := emit(Table("release "+release.ID, []string{"FIELD", "VALUE"}, [][]string{
			{"project", release.Project}, {"state", string(release.Status)}, {"trigger", string(release.Trigger)},
			{"branch", release.Branch}, {"revision", release.Commit}, {"image", release.Image},
			{"duration", duration(release.Duration)}, {"started", release.StartedAt.Local().Format(time.RFC3339)},
			{"message", release.Message},
		})); err != nil {
			return err
		}
		rows := make([][]string, 0, len(release.Targets))
		for _, target := range release.Targets {
			rows = append(rows, []string{target.AgentName, string(target.Status), target.Message})
		}
		return emit(Table("targets", []string{"AGENT", "STATE", "DETAIL"}, rows))
	case "follow":
		return e.followRelease(ctx, args[1], emit)
	case "logs":
		if contains(args[2:], "--follow") {
			return e.followRelease(ctx, args[1], emit)
		}
		return e.emitLogs(args[1], 0, emit)
	default:
		return fmt.Errorf("unknown release command %q", args[0])
	}
}

func (e *Engine) emitLogs(releaseID string, after int64, emit Emit) error {
	logs, err := e.server.Store().ListLogs(releaseID)
	if err != nil {
		return err
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].ID < logs[j].ID })
	for _, line := range logs {
		if line.ID <= after {
			continue
		}
		level := "info"
		if line.Stream == "stderr" {
			level = "warning"
		}
		if err := emit(Event{Type: EventLog, Time: line.Timestamp, Level: level, Operation: releaseID,
			Title: line.AgentName, Message: line.Line, Data: map[string]any{"id": line.ID, "stage": line.Stage, "stream": line.Stream}}); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) followRelease(ctx context.Context, releaseID string, emit Emit) error {
	var lastLog int64
	var lastStatus models.Status
	ticker := time.NewTicker(followInterval)
	defer ticker.Stop()
	for {
		logs, err := e.server.Store().ListLogs(releaseID)
		if err != nil {
			return err
		}
		sort.Slice(logs, func(i, j int) bool { return logs[i].ID < logs[j].ID })
		for _, line := range logs {
			if line.ID <= lastLog {
				continue
			}
			lastLog = line.ID
			level := "info"
			if line.Stream == "stderr" {
				level = "warning"
			}
			if err := emit(Event{Type: EventLog, Time: line.Timestamp, Level: level, Operation: releaseID,
				Title: line.AgentName, Message: line.Line, Data: map[string]any{"id": line.ID, "stage": line.Stage}}); err != nil {
				return err
			}
		}
		release, err := e.server.Store().GetRelease(releaseID)
		if err != nil {
			return err
		}
		if release.Status != lastStatus {
			lastStatus = release.Status
			level := "info"
			if release.Status == models.StatusSucceeded {
				level = "success"
			} else if release.Status == models.StatusFailed {
				level = "error"
			}
			message := fmt.Sprintf("%s · %s", release.Project, release.Status)
			if release.Message != "" {
				message += " · " + release.Message
			}
			if err := emit(Event{Type: EventMessage, Time: time.Now(), Level: level, Operation: releaseID, Message: message}); err != nil {
				return err
			}
		}
		if release.Status.Done() {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (e *Engine) alerts(args []string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		values, err := e.server.Store().ListActiveAlerts()
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, alert := range values {
			rows = append(rows, []string{alert.ID, string(alert.Severity), alert.AgentName, alert.Type, alert.Message, since(alert.CreatedAt)})
		}
		return emit(Table("alerts", []string{"ID", "SEVERITY", "AGENT", "TYPE", "MESSAGE", "AGE"}, rows))
	}
	if args[0] != "resolve" || len(args) != 2 {
		return errors.New("usage: alert list | alert resolve ID")
	}
	if err := e.server.Store().ResolveAlert(args[1]); err != nil {
		return err
	}
	return emit(Message("success", "resolved alert "+args[1]))
}

func (e *Engine) registry(ctx context.Context, args []string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		values, err := e.server.Registry().Images(ctx)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, image := range values {
			rows = append(rows, []string{image.Repository, image.Tag, short(image.Digest, 20), bytes(uint64(max(image.Size, 0))), since(image.CreatedAt)})
		}
		return emit(Table("registry", []string{"REPOSITORY", "TAG", "DIGEST", "SIZE", "AGE"}, rows))
	}
	if args[0] != "remove" || len(args) != 3 {
		return errors.New("usage: registry list | registry remove REPOSITORY TAG")
	}
	if err := e.server.Registry().DeleteTag(ctx, args[1], args[2]); err != nil {
		return err
	}
	return emit(Message("success", "deleted manifest "+args[1]+":"+args[2]))
}

func (e *Engine) secrets(args []string, input string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		values, err := e.server.Store().ListSecrets()
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, secret := range values {
			rows = append(rows, []string{secret.Name, "${secret:" + secret.Name + "}", since(secret.UpdatedAt)})
		}
		return emit(Table("secrets", []string{"NAME", "REFERENCE", "UPDATED"}, rows))
	}
	if len(args) != 2 {
		return errors.New("usage: secret list | secret set NAME | secret remove NAME")
	}
	switch args[0] {
	case "set":
		input = strings.TrimSuffix(input, "\n")
		if input == "" {
			return errors.New("secret value must be supplied on stdin")
		}
		sealed, err := e.server.Vault().Seal(input)
		if err != nil {
			return err
		}
		if err := e.server.Store().SetSecret(args[1], sealed); err != nil {
			return err
		}
		return emit(Message("success", "stored "+args[1]+" · ${secret:"+args[1]+"}"))
	case "remove":
		if err := e.server.Store().DeleteSecret(args[1]); err != nil {
			return err
		}
		return emit(Message("success", "removed secret "+args[1]))
	default:
		return fmt.Errorf("unknown secret command %q", args[0])
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
