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

import "strings"

type commandSpec struct {
	Command   string
	Summary   string
	NeedsArgs bool
}

var commandCatalog = []commandSpec{
	{Command: "status", Summary: "Fleet health, agents and projects"},
	{Command: "events", Summary: "Follow new activity across the fleet"},
	{Command: "deploy", Summary: "Start and follow a project release", NeedsArgs: true},
	{Command: "rollback", Summary: "Restore a project's previous release", NeedsArgs: true},
	{Command: "logs", Summary: "Read or follow release output", NeedsArgs: true},
	{Command: "stop", Summary: "Stop a project on every runner", NeedsArgs: true},
	{Command: "agent list", Summary: "List enrolled agents"},
	{Command: "agent show", Summary: "Inspect an agent", NeedsArgs: true},
	{Command: "agent add", Summary: "Enrol a new agent", NeedsArgs: true},
	{Command: "agent remove", Summary: "Remove an enrolled agent", NeedsArgs: true},
	{Command: "project list", Summary: "List YAML-owned projects"},
	{Command: "project show", Summary: "Inspect a project and its services", NeedsArgs: true},
	{Command: "project edit", Summary: "Open authoritative YAML", NeedsArgs: true},
	{Command: "project validate", Summary: "Validate environment YAML", NeedsArgs: true},
	{Command: "project apply", Summary: "Validate and atomically apply YAML", NeedsArgs: true},
	{Command: "project reload", Summary: "Reload project files"},
	{Command: "project deploy", Summary: "Start and follow a release", NeedsArgs: true},
	{Command: "project rollback", Summary: "Restore the previous release", NeedsArgs: true},
	{Command: "project stop", Summary: "Stop a project on every runner", NeedsArgs: true},
	{Command: "release list", Summary: "List recent releases"},
	{Command: "release show", Summary: "Inspect a release", NeedsArgs: true},
	{Command: "release logs", Summary: "Read or follow release output", NeedsArgs: true},
	{Command: "release follow", Summary: "Attach to a release", NeedsArgs: true},
	{Command: "container list", Summary: "List managed containers"},
	{Command: "container logs", Summary: "Stream application output", NeedsArgs: true},
	{Command: "registry list", Summary: "List stored image manifests"},
	{Command: "registry remove", Summary: "Delete an image manifest", NeedsArgs: true},
	{Command: "alert list", Summary: "List active alerts"},
	{Command: "alert resolve", Summary: "Resolve an alert", NeedsArgs: true},
	{Command: "secret list", Summary: "List encrypted secret names"},
	{Command: "secret set", Summary: "Store a value using masked input", NeedsArgs: true},
	{Command: "secret remove", Summary: "Remove an encrypted secret", NeedsArgs: true},
	{Command: "help", Summary: "Explain every workspace command"},
	{Command: "clear", Summary: "Clear the response transcript"},
	{Command: "exit", Summary: "Close the workspace"},
}

func matchingCommands(value string, limit int) []commandSpec {
	hadTrailingSpace := strings.HasSuffix(value, " ") || strings.HasSuffix(value, "\t")
	raw := strings.TrimSpace(value)
	palette := strings.HasPrefix(raw, "/")
	query := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "/")))
	if query == "show" {
		query = ""
		palette = true
	}
	if query == "" && !palette {
		return nil
	}
	if hadTrailingSpace {
		for _, item := range commandCatalog {
			if strings.EqualFold(query, item.Command) && item.NeedsArgs {
				return nil
			}
		}
	}

	// Once arguments begin, command discovery has done its job and gets out
	// of the way of normal input.
	for _, item := range commandCatalog {
		if item.NeedsArgs && strings.HasPrefix(query, item.Command+" ") {
			return nil
		}
	}

	var prefix, word, contains []commandSpec
	for _, item := range commandCatalog {
		command := strings.ToLower(item.Command)
		summary := strings.ToLower(item.Summary)
		switch {
		case query == "" || strings.HasPrefix(command, query):
			prefix = append(prefix, item)
		case wordHasPrefix(command, query):
			word = append(word, item)
		case strings.Contains(command, query) || strings.Contains(summary, query):
			contains = append(contains, item)
		}
	}
	result := append(prefix, word...)
	result = append(result, contains...)
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func wordHasPrefix(value, query string) bool {
	for _, field := range strings.Fields(value) {
		if strings.HasPrefix(field, query) {
			return true
		}
	}
	return false
}
