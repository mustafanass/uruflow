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
	"strconv"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/projects"
)

func (e *Engine) projects(ctx context.Context, args []string, input string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		values, err := e.server.Store().ListProjects()
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, project := range values {
			rows = append(rows, []string{project.Name, project.Env, project.EffectiveWorkflow(),
				strconv.Itoa(len(project.ServiceList())), project.Source})
		}
		return emit(Table("projects", []string{"NAME", "ENV", "WORKFLOW", "SERVICES", "SOURCE"}, rows))
	}
	switch args[0] {
	case "variables", "variables-source":
		if len(args) != 2 {
			return grammar.UsageError("project", args[0])
		}
		return e.projectVariables(args[1], input, args[0] == "variables-source", emit)
	case "create":
		if len(args) != 3 {
			return grammar.UsageError("project", "create")
		}
		return e.createProject(args[1], args[2], input, emit)
	case "edit":
		if len(args) != 2 {
			return grammar.UsageError("project", "edit")
		}
		return e.editProject(args[1], input, emit)
	case "show":
		if len(args) != 2 {
			return grammar.UsageError("project", "show")
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
			{"source", project.Source}, {"workflow", project.EffectiveWorkflow()},
			{"timeout", project.EffectiveTimeout().String()},
			{"builder", builder}, {"runners", strings.Join(runners, ", ")},
			{"services", strconv.Itoa(len(project.ServiceList()))},
		})); err != nil {
			return err
		}
		rows := make([][]string, 0, len(project.ServiceList()))
		for _, service := range project.ServiceList() {
			source := service.Image
			branch := ""
			build := ""
			if service.Built() {
				source, branch, build = service.GitURL, service.Branch, service.BuildFile()
			}
			rows = append(rows, []string{service.Name, service.EffectiveMode(), source, branch, build,
				strings.Join(models.FormatPorts(service.Ports), ","), service.Restart})
		}
		return emit(Table("services", []string{"NAME", "MODE", "SOURCE", "BRANCH", "BUILD", "PORTS", "RESTART"}, rows))
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
			return grammar.UsageError("project", args[0])
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
			return grammar.UsageError("project", "stop")
		}
		if err := e.server.Pipeline().Stop(args[1]); err != nil {
			return err
		}
		return emit(Message("success", args[1]+" stopped on every runner"))
	default:
		return fmt.Errorf("unknown project command %q", args[0])
	}
}

func (e *Engine) createProject(project, environment, input string, emit Emit) error {
	if !models.ValidResourceName(project) || !models.ValidResourceName(environment) {
		return errors.New("project and environment must be lowercase resource names")
	}
	environmentDefinition, err := projects.ParseCreationYAML(input)
	if err != nil {
		return fmt.Errorf("validate environment YAML: %w", err)
	}
	draft := projects.Draft{
		Project: project, Env: environment, Environment: environmentDefinition,
	}
	environmentPath, _ := e.server.Loader().Paths(project, environment)
	_, projectErr := os.Stat(filepath.Dir(environmentPath))
	newProject := errors.Is(projectErr, os.ErrNotExist)
	if projectErr != nil && !newProject {
		return projectErr
	}
	if err := e.server.Loader().Create(draft); err != nil {
		return err
	}
	loaded := e.server.ReloadProjects()
	projectDir := filepath.Dir(environmentPath)
	for _, problem := range e.server.ProjectProblems() {
		if problem.Path == environmentPath || newProject && pathWithin(projectDir, problem.Path) {
			_ = e.server.Loader().Remove(project, environment)
			e.server.ReloadProjects()
			return fmt.Errorf("project was not created: %w", problem.Reason)
		}
	}
	return emit(Event{Type: EventResult, Time: time.Now(), Title: "project created", Data: map[string]any{
		"project": project, "environment": environment, "services": len(environmentDefinition.Services),
		"environment_file": environmentPath, "loaded": loaded,
	}})
}

func pathWithin(directory, path string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func (e *Engine) editProject(name, input string, emit Emit) error {
	project, err := e.server.Store().GetProject(name)
	if err != nil {
		return fmt.Errorf("project %s: %w", name, err)
	}
	if !project.Managed() || project.Env == "" {
		return fmt.Errorf("project %s has no authoritative environment file", project.Name)
	}
	if input == "" {
		content, err := os.ReadFile(project.Source)
		if err != nil {
			return err
		}
		return emit(Event{Type: EventResult, Time: time.Now(), Title: "project YAML editor", Message: string(content)})
	}
	if err := e.server.Loader().Validate(project.Source, input); err != nil {
		return fmt.Errorf("validate YAML: %w", err)
	}
	previous, err := os.ReadFile(project.Source)
	if err != nil {
		return err
	}
	if err := e.server.Loader().WriteEnvironment(project.Base(), project.Env, []byte(input)); err != nil {
		return err
	}
	loaded := e.server.ReloadProjects()
	for _, problem := range e.server.ProjectProblems() {
		if problem.Path == project.Source {
			restoreErr := e.server.Loader().WriteEnvironment(project.Base(), project.Env, previous)
			e.server.ReloadProjects()
			if restoreErr != nil {
				return fmt.Errorf("YAML was not saved: %v; restore previous YAML: %w", problem.Reason, restoreErr)
			}
			return fmt.Errorf("YAML was not saved: %w", problem.Reason)
		}
	}
	return emit(Message("success", fmt.Sprintf("saved %s and loaded %d project environments", project.Source, loaded)))
}
