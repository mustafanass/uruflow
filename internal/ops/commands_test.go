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
	"path/filepath"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/activity"
	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/models"
)

func TestHelpIsGeneratedFromTheCanonicalGrammar(t *testing.T) {
	event := (&Engine{}).help()
	commands := grammar.Visible()
	if len(event.Rows) != len(commands) {
		t.Fatalf("help rows = %d, grammar commands = %d", len(event.Rows), len(commands))
	}
	for index, command := range commands {
		if event.Rows[index][0] != grammar.Usage(command) || event.Rows[index][1] != command.Summary {
			t.Fatalf("row %d = %#v, command = %#v", index, event.Rows[index], command)
		}
	}
}

func TestAgentRolesAreExplicitAndCanonical(t *testing.T) {
	for _, test := range []struct {
		options []string
		want    string
	}{
		{want: "runner"},
		{options: []string{"--roles", "builder"}, want: "builder"},
		{options: []string{"--roles", "builder,runner"}, want: "builder,runner"},
	} {
		_, got, err := parseAgentRoles(test.options)
		if err != nil {
			t.Fatalf("parse %v: %v", test.options, err)
		}
		if got != test.want {
			t.Fatalf("parse %v = %q, want %q", test.options, got, test.want)
		}
	}

	for _, options := range [][]string{{"builder"}, {"--role", "builder"}, {"--roles"}, {"--roles", "builder", "extra"}} {
		if _, _, err := parseAgentRoles(options); err == nil {
			t.Fatalf("parse %v unexpectedly succeeded", options)
		}
	}
}

func TestActivityEntryKeepsItsSequenceAndShape(t *testing.T) {
	stamp := time.Now()
	event := activityEvent(activity.Entry{Sequence: 42, Time: stamp, Kind: activity.KindLog,
		Level: "warning", Operation: "r1", Source: "builder-01", Message: "warning"})
	if event.Type != EventLog || event.Sequence != 42 || event.Time != stamp || event.Operation != "r1" || event.Title != "builder-01" {
		t.Fatalf("event=%+v", event)
	}
}

func TestPathWithinOnlyMatchesTheCreatedProject(t *testing.T) {
	directory := filepath.Join("var", "lib", "uruflow", "projects", "api")
	if !pathWithin(directory, filepath.Join(directory, "prod.yaml")) {
		t.Fatal("project environment was not recognized")
	}
	if pathWithin(directory, filepath.Join("var", "lib", "uruflow", "projects", "web", "prod.yaml")) {
		t.Fatal("another project's problem was included")
	}
}

func TestContainerKindDistinguishesSystemAndProjectWorkloads(t *testing.T) {
	for _, test := range []struct {
		container models.Container
		want      string
	}{
		{container: models.Container{Name: "registry"}, want: "system"},
		{container: models.Container{Name: "api", Project: "api-prod", Service: "api"}, want: "service"},
	} {
		if got := containerKind(test.container); got != test.want {
			t.Fatalf("containerKind(%#v) = %q, want %q", test.container, got, test.want)
		}
	}
}
