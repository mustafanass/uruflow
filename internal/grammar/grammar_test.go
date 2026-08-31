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

package grammar

import (
	"strings"
	"testing"
)

func TestCanonicalCommandsResolve(t *testing.T) {
	valid := [][]string{
		{"status"},
		{"events"},
		{"events", "--after", "42"},
		{"agent", "add", "builder-01"},
		{"agent", "add", "builder-01", "--roles", "builder,runner"},
		{"project", "create", "api", "stg"},
		{"project", "deploy", "api-stg", "--no-follow"},
		{"release", "list", "--limit", "100"},
		{"container", "logs", "runner-01", "abc123", "--tail", "250", "--follow"},
	}
	for _, args := range valid {
		if _, err := Resolve(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
}

func TestDuplicateAliasesAreRejected(t *testing.T) {
	invalid := [][]string{
		{"deploy", "api-stg"},
		{"rollback", "api-stg"},
		{"logs", "release-id"},
		{"agents", "list"},
		{"projects", "list"},
		{"show"},
		{"quit"},
	}
	for _, args := range invalid {
		if _, err := Resolve(args); err == nil {
			t.Errorf("%v unexpectedly resolved", args)
		}
	}
}

func TestUsageComesFromTheSchema(t *testing.T) {
	_, err := Resolve([]string{"project", "create", "api"})
	if err == nil || err.Error() != "usage: project create PROJECT ENV" {
		t.Fatalf("error = %v", err)
	}
	command, ok := Find("agent", "add")
	if !ok || Usage(command) != "agent add NAME [--roles builder|runner|builder,runner]" {
		t.Fatalf("usage = %q", Usage(command))
	}
}

func TestVisibleCommandsHaveUniquePaths(t *testing.T) {
	seen := make(map[string]bool)
	for _, command := range Visible() {
		path := Path(command)
		if path == "" || seen[path] {
			t.Fatalf("duplicate or empty command path %q", path)
		}
		seen[path] = true
		if strings.TrimSpace(command.Summary) == "" {
			t.Fatalf("%s has no summary", path)
		}
	}
}

func TestInteractionMetadataLivesWithTheCommand(t *testing.T) {
	for _, test := range []struct {
		path    []string
		input   InputMode
		confirm bool
		durable bool
		focused bool
	}{
		{path: []string{"secret", "set"}, input: InputSecret},
		{path: []string{"project", "create"}, input: InputYAML},
		{path: []string{"agent", "remove"}, confirm: true},
		{path: []string{"project", "stop"}, confirm: true},
		{path: []string{"registry", "remove"}, confirm: true},
		{path: []string{"alert", "resolve"}, confirm: true},
		{path: []string{"secret", "remove"}, confirm: true},
		{path: []string{"project", "deploy"}, durable: true},
		{path: []string{"project", "rollback"}, durable: true},
		{path: []string{"events"}, focused: true},
	} {
		command, found := Find(test.path...)
		if !found || command.Input != test.input || command.Confirm != test.confirm ||
			command.Durable != test.durable || command.Focused != test.focused {
			t.Fatalf("%v metadata = %+v", test.path, command)
		}
	}
}
