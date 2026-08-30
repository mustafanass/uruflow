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

package workbench

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/ops"
)

type argumentContext struct {
	Key      string
	Prefix   string
	Query    string
	Label    string
	Request  []string
	Items    []commandSpec
	EmptyTip string
}

type completionMsg struct {
	key   string
	items []commandSpec
	err   error
}

var argumentRules = []struct {
	command string
	key     string
	label   string
	request []string
	empty   string
}{
	{"project rollback", "projects", "PROJECTS", []string{"project", "list"}, "No loaded projects. Add project YAML, then run project reload."},
	{"project deploy", "projects", "PROJECTS", []string{"project", "list"}, "No loaded projects. Add project YAML, then run project reload."},
	{"project show", "projects", "PROJECTS", []string{"project", "list"}, "No loaded projects. Add project YAML, then run project reload."},
	{"project edit", "projects", "PROJECTS", []string{"project", "list"}, "No loaded projects. Add project YAML, then run project reload."},
	{"project stop", "projects", "PROJECTS", []string{"project", "list"}, "No loaded projects. Add project YAML, then run project reload."},
	{"release follow", "releases", "RELEASES", []string{"release", "list", "--limit", "100"}, "No releases have been recorded yet."},
	{"release logs", "releases", "RELEASES", []string{"release", "list", "--limit", "100"}, "No releases have been recorded yet."},
	{"release show", "releases", "RELEASES", []string{"release", "list", "--limit", "100"}, "No releases have been recorded yet."},
	{"agent remove", "agents", "AGENTS", []string{"agent", "list"}, "No agents are enrolled."},
	{"agent show", "agents", "AGENTS", []string{"agent", "list"}, "No agents are enrolled."},
	{"agent add", "new-agent", "NEW AGENT", nil, "Type a lowercase agent name, then press Enter."},
	{"secret set", "new-secret", "SECRET NAME", nil, "Type a secret name; its value is entered securely next."},
	{"project apply", "project-yaml", "PROJECT YAML", nil, "Type PROJECT ENVIRONMENT -, then press Enter for the inline editor."},
	{"project validate", "yaml-file", "YAML FILE", nil, "Type a YAML path, or - to open the inline editor."},
	{"rollback", "projects", "PROJECTS", []string{"project", "list"}, "No loaded projects. Add project YAML, then run project reload."},
	{"deploy", "projects", "PROJECTS", []string{"project", "list"}, "No loaded projects. Add project YAML, then run project reload."},
	{"stop", "projects", "PROJECTS", []string{"project", "list"}, "No loaded projects. Add project YAML, then run project reload."},
	{"logs", "releases", "RELEASES", []string{"release", "list", "--limit", "100"}, "No releases have been recorded yet."},
}

func argumentCompletion(value string) (argumentContext, bool) {
	raw := strings.TrimLeft(value, " \t")
	lower := strings.ToLower(raw)
	if completion, ok := agentRoleCompletion(raw, lower); ok {
		return completion, true
	}
	for _, rule := range argumentRules {
		prefix := rule.command + " "
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		remainder := raw[len(prefix):]
		if strings.ContainsAny(strings.TrimSpace(remainder), " \t") {
			return argumentContext{}, false
		}
		return argumentContext{
			Key: rule.key, Prefix: prefix, Query: strings.TrimSpace(remainder), Label: rule.label,
			Request: rule.request, EmptyTip: rule.empty,
		}, true
	}
	return argumentContext{}, false
}

func agentRoleCompletion(raw, lower string) (argumentContext, bool) {
	const command = "agent add "
	if !strings.HasPrefix(lower, command) {
		return argumentContext{}, false
	}
	remainder := raw[len(command):]
	separator := strings.IndexAny(remainder, " \t")
	name, options := remainder, ""
	if separator >= 0 {
		name = remainder[:separator]
		options = strings.TrimSpace(remainder[separator:])
	}
	if name == "" {
		return argumentContext{}, false
	}
	query := ""
	if options != "" {
		fields := strings.Fields(options)
		if len(fields) > 2 || fields[0] != "--roles" {
			return argumentContext{}, false
		}
		if len(fields) == 2 {
			query = fields[1]
		}
	}
	prefix := command + name + " --roles "
	return argumentContext{
		Key: "agent-roles", Prefix: prefix, Query: query, Label: "AGENT ROLE",
		Items: []commandSpec{
			{Command: prefix + "runner", Summary: "Run deployed containers"},
			{Command: prefix + "builder", Summary: "Build and publish images"},
			{Command: prefix + "builder,runner", Summary: "Build and run on this machine"},
		},
		EmptyTip: "Choose builder, runner, or builder,runner.",
	}, true
}

func (m *model) requestArgumentCompletion() tea.Cmd {
	completion, ok := argumentCompletion(m.input.Value())
	if !ok || m.client == nil {
		return nil
	}
	if len(completion.Items) > 0 {
		return nil
	}
	if len(completion.Request) == 0 {
		return nil
	}
	if m.completionLoading == nil {
		m.completionLoading = make(map[string]bool)
	}
	if m.completionCache == nil {
		m.completionCache = make(map[string][]commandSpec)
	}
	if m.completionLoading[completion.Key] {
		return nil
	}
	if _, cached := m.completionCache[completion.Key]; cached {
		return nil
	}
	m.completionLoading[completion.Key] = true
	return func() tea.Msg {
		var items []commandSpec
		err := m.client.Execute(context.Background(), completion.Request, "", func(event ops.Event) error {
			if event.Type != ops.EventTable {
				return nil
			}
			for _, row := range event.Rows {
				if len(row) == 0 || row[0] == "" {
					continue
				}
				items = append(items, commandSpec{
					Command: completion.Prefix + row[0],
					Summary: completionSummary(event.Columns, row),
				})
			}
			return nil
		})
		return completionMsg{key: completion.Key, items: items, err: err}
	}
}

func completionSummary(columns, row []string) string {
	preferred := []string{"PROJECT", "WORKFLOW", "ROLES", "STATE", "ENV", "BRANCH", "AGE"}
	var values []string
	for _, wanted := range preferred {
		for index, column := range columns {
			if index == 0 || index >= len(row) || column != wanted || row[index] == "" || row[index] == "–" {
				continue
			}
			values = append(values, row[index])
			break
		}
		if len(values) == 2 {
			break
		}
	}
	return strings.Join(values, " · ")
}

func filterArgumentCommands(items []commandSpec, completion argumentContext) []commandSpec {
	query := strings.ToLower(completion.Query)
	if query == "" {
		return items
	}
	filtered := make([]commandSpec, 0, len(items))
	for _, item := range items {
		value := strings.TrimPrefix(item.Command, completion.Prefix)
		if strings.HasPrefix(strings.ToLower(value), query) || strings.Contains(strings.ToLower(item.Summary), query) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
