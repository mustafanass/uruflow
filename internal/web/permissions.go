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

package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/observe"
	"github.com/mustafanass/uruflow/internal/secrets"
	"github.com/mustafanass/uruflow/internal/storage"
)

func visibleSnapshot(s observe.Snapshot, u *storage.ConsoleUser) observe.Snapshot {
	if u.AllProjects() {
		return s
	}
	v := observe.Snapshot{At: s.At, Version: s.Version, ServerName: s.ServerName, Registry: s.Registry,
		Graphs: map[string]observe.ProjectGraphs{}, ProjectStates: map[string]string{}}
	agents, operations := map[string]bool{}, map[string]bool{}
	addAgent := func(id string) {
		if id != "" {
			agents[id] = true
		}
	}
	for _, p := range s.Projects {
		if u.CanProject(p.Name) {
			v.Projects = append(v.Projects, p)
			addAgent(p.Builder)
			for _, id := range p.Runners {
				addAgent(id)
			}
			v.Graphs[p.Name] = s.Graphs[p.Name]
			v.ProjectStates[p.Name] = s.ProjectStates[p.Name]
		}
	}
	for _, r := range s.Releases {
		if u.CanProject(r.Project) {
			v.Releases = append(v.Releases, r)
			operations[r.ID] = true
			addAgent(r.Builder)
			for _, target := range r.Targets {
				addAgent(target.AgentID)
			}
		}
	}
	for _, c := range s.Containers {
		if c.Project != "" && u.CanProject(c.Project) {
			v.Containers = append(v.Containers, c)
			addAgent(c.AgentID)
		}
	}
	for _, a := range s.Agents {
		if agents[a.ID] {
			v.Agents = append(v.Agents, a)
		}
	}

	for _, a := range s.Alerts {
		if a.Project != "" && u.CanProject(a.Project) {
			v.Alerts = append(v.Alerts, a)
			operations[a.ID] = true
		}
	}
	for _, entry := range s.Activity {
		if operations[entry.Operation] {
			v.Activity = append(v.Activity, entry)
		}
	}
	v.Fleet = v.FleetGraph()
	return v
}
func requireProject(w http.ResponseWriter, u *storage.ConsoleUser, name string) bool {
	if name == "" || !u.CanProject(name) {
		writeError(w, 404, "Project not found.")
		return false
	}
	return true
}
func requireFleet(w http.ResponseWriter, u *storage.ConsoleUser) bool {
	if !u.AllProjects() {
		writeError(w, 403, "This action requires access to all projects.")
		return false
	}
	return true
}
func validProjects(projects []string) ([]string, bool) {
	if len(projects) > 256 {
		return nil, false
	}
	unique := map[string]bool{}
	for _, p := range projects {
		if p != "*" && !models.ValidResourceName(p) {
			return nil, false
		}
		unique[p] = true
	}
	if unique["*"] {
		return []string{"*"}, true
	}
	result := make([]string, 0, len(unique))
	for p := range unique {
		result = append(result, p)
	}
	sort.Strings(result)
	return result, true
}

var errForbidden = errors.New("permission denied")

func (h *Handler) documentPolicy(u *storage.ConsoleUser, name string) (func(models.Project) error, error) {
	if u.AllProjects() {
		return nil, nil
	}
	h.mu.Lock()
	s := h.latest
	h.mu.Unlock()
	if s == nil {
		return nil, errors.New("waiting for project permissions to be observed")
	}
	visible := visibleSnapshot(*s, u)
	allowedSecrets, allowedAgents := map[string]bool{}, map[string]bool{}
	for _, p := range visible.Projects {
		for _, ref := range secrets.Names(p.Runtime.Env) {
			allowedSecrets[ref] = true
		}
		for _, service := range p.ServiceList() {
			for _, ref := range secrets.Names(service.Env) {
				allowedSecrets[ref] = true
			}
		}
	}
	for _, a := range visible.Agents {
		allowedAgents[a.ID] = true
	}
	return func(p models.Project) error {
		if p.Name != name {
			return fmt.Errorf("%w: project identity cannot change", errForbidden)
		}
		requested := append([]string{}, p.Runners...)
		if p.Builder != "" {
			requested = append(requested, p.Builder)
		}
		for _, id := range requested {
			if !allowedAgents[id] {
				return fmt.Errorf("%w: ask a fleet operator to assign additional agents to this project", errForbidden)
			}
		}

		resolved, _ := json.Marshal(p)
		for _, ref := range secrets.References(string(resolved)) {
			if !strings.HasPrefix(ref, name+".") && !allowedSecrets[ref] {
				return fmt.Errorf("%w: this secret reference is outside your project access", errForbidden)
			}
		}
		return nil
	}, nil
}
