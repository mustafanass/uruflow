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
				strconv.Itoa(len(project.ServiceList())), project.Branch, project.Source})
		}
		return emit(Table("projects", []string{"NAME", "ENV", "WORKFLOW", "SERVICES", "BRANCH", "SOURCE"}, rows))
	}
	switch args[0] {
	case "create":
		if len(args) != 3 {
			return grammar.UsageError("project", "create")
		}
		return e.createProject(args[1], args[2], input, emit)
	case "path":
		if len(args) != 2 {
			return grammar.UsageError("project", "path")
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
	case "validate":
		if len(args) != 2 {
			return grammar.UsageError("project", "validate")
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
			return grammar.UsageError("project", "apply")
		}
		return e.applyProject(args[1], args[2], args[3], input, emit)
	default:
		return fmt.Errorf("unknown project command %q", args[0])
	}
}

func (e *Engine) createProject(project, environment, input string, emit Emit) error {
	if !models.ValidResourceName(project) || !models.ValidResourceName(environment) {
		return errors.New("project and environment must be lowercase resource names")
	}
	document, err := projects.ParseCreationYAML(input)
	if err != nil {
		return fmt.Errorf("validate project YAML: %w", err)
	}
	if document.Project.Name != "" && document.Project.Name != project {
		return fmt.Errorf("project.name %q must match %q", document.Project.Name, project)
	}
	document.Project.Name = project
	draft := projects.Draft{
		Project: project, Env: environment,
		Definition: document.Project, Environment: document.Environment,
	}
	if err := e.server.Loader().Create(draft); err != nil {
		return err
	}
	definitionPath, environmentPath, _ := e.server.Loader().Paths(project, environment)
	loaded := e.server.ReloadProjects()
	projectDir := filepath.Dir(definitionPath)
	for _, problem := range e.server.ProjectProblems() {
		if pathWithin(projectDir, problem.Path) {
			_ = e.server.Loader().Remove(project, environment)
			e.server.ReloadProjects()
			return fmt.Errorf("project was not created: %w", problem.Reason)
		}
	}
	return emit(Event{Type: EventResult, Time: time.Now(), Title: "project created", Data: map[string]any{
		"project": project, "environment": environment, "services": len(document.Environment.Services),
		"project_file": definitionPath, "environment_file": environmentPath, "loaded": loaded,
	}})
}

func pathWithin(directory, path string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
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
