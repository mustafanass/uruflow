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
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestCommandLineSupportsQuotesAndAliases(t *testing.T) {
	args, err := split(`project show "api prod"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || args[2] != "api prod" {
		t.Fatalf("args = %#v", args)
	}
	normalized := normalize([]string{"deploy", "api-prod"})
	if len(normalized) != 3 || normalized[0] != "project" || normalized[1] != "deploy" {
		t.Fatalf("normalized = %#v", normalized)
	}
	if help := normalize([]string{"show"}); len(help) != 1 || help[0] != "help" {
		t.Fatalf("show normalized = %#v", help)
	}
}

func TestApplyFromDashEntersPasteMode(t *testing.T) {
	if !wantsPaste([]string{"project", "apply", "api", "prod", "-"}) {
		t.Fatal("project apply from stdin did not enter paste mode")
	}
	if wantsPaste([]string{"project", "apply", "api", "prod", "file.yaml"}) {
		t.Fatal("file apply unexpectedly entered paste mode")
	}
}

func TestSensitiveAndDestructiveCommandsStayInteractive(t *testing.T) {
	if !wantsSecret([]string{"secret", "set", "database-password"}) {
		t.Fatal("secret set did not enter masked input mode")
	}
	if wantsSecret([]string{"secret", "list"}) {
		t.Fatal("secret list unexpectedly entered masked input mode")
	}
	for _, args := range [][]string{
		{"agent", "remove", "old-runner"},
		{"project", "stop", "api-prod"},
		{"registry", "remove", "api", "stale"},
		{"secret", "remove", "old-token"},
	} {
		if !needsConfirmation(args) {
			t.Fatalf("%v did not require confirmation", args)
		}
	}
}

func TestOnlyReleaseCommandsAreDurableAfterDetach(t *testing.T) {
	if !isDurableOperation([]string{"project", "deploy", "api-prod"}) {
		t.Fatal("deploy should continue after detaching")
	}
	if isDurableOperation([]string{"events"}) {
		t.Fatal("fleet event subscription is not a durable operation")
	}
}

func TestEventsUseAFocusedStreamingPage(t *testing.T) {
	if !isFocusedStream([]string{"events"}) || isFocusedStream([]string{"status"}) {
		t.Fatal("focused stream command detection is incorrect")
	}
	for _, width := range []int{24, 60, 100} {
		m := &model{width: width}
		for row, line := range strings.Split(m.streamWelcome(), "\n") {
			if got := utf8Width(line); got > width {
				t.Fatalf("width %d row %d uses %d columns", width, row+1, got)
			}
		}
	}
}

func TestCommandSuggestionsFilterAsTheUserTypes(t *testing.T) {
	matches := matchingCommands("s", 0)
	if len(matches) == 0 || matches[0].Command != "status" {
		t.Fatalf("matches = %#v", matches)
	}
	project := matchingCommands("project dep", 0)
	if len(project) != 1 || project[0].Command != "project deploy" {
		t.Fatalf("project matches = %#v", project)
	}
	if got := matchingCommands("project deploy api-prod", 0); len(got) != 0 {
		t.Fatalf("suggestions remained visible while entering arguments: %#v", got)
	}
	if got := matchingCommands("agent add ", 0); len(got) != 0 {
		t.Fatalf("command suggestion repeated after its argument began: %#v", got)
	}
}

func TestDeployAdvancesFromCommandToLoadedProjects(t *testing.T) {
	completion, ok := argumentCompletion("deploy ")
	if !ok || completion.Key != "projects" || completion.Query != "" {
		t.Fatalf("completion = %#v, %v", completion, ok)
	}
	input := textinput.New()
	input.SetValue("deploy a")
	m := &model{
		input: input,
		completionCache: map[string][]commandSpec{"projects": {
			{Command: "deploy api-prod", Summary: "build_deploy · prod"},
			{Command: "deploy web-prod", Summary: "deploy_only · prod"},
		}},
	}
	matches := m.suggestions()
	if len(matches) != 1 || matches[0].Command != "deploy api-prod" {
		t.Fatalf("project matches = %#v", matches)
	}
	if _, ok := argumentCompletion("deploy api prod"); ok {
		t.Fatal("completion stayed open after the first argument")
	}
}

func TestCreationCommandsAdvanceToGuidedInput(t *testing.T) {
	for _, test := range []struct {
		value string
		label string
	}{
		{"agent add ", "NEW AGENT"},
		{"secret set ", "SECRET NAME"},
		{"project apply ", "PROJECT YAML"},
		{"project validate ", "YAML FILE"},
	} {
		completion, ok := argumentCompletion(test.value)
		if !ok || completion.Label != test.label || completion.EmptyTip == "" {
			t.Fatalf("%q completion = %#v, %v", test.value, completion, ok)
		}
	}
}

func TestAgentEnrollmentAdvancesFromNameToRole(t *testing.T) {
	input := textinput.New()
	input.SetValue("agent add build-01")
	m := &model{input: input}

	completion, ok := argumentCompletion(input.Value())
	if !ok || completion.Label != "AGENT ROLE" || len(completion.Items) != 3 {
		t.Fatalf("completion = %#v, %v", completion, ok)
	}
	matches := m.suggestions()
	if len(matches) != 3 {
		t.Fatalf("role suggestions = %#v", matches)
	}
	if matches[0].Command != "agent add build-01 --roles runner" || matches[2].Command != "agent add build-01 --roles builder,runner" {
		t.Fatalf("role suggestions = %#v", matches)
	}
	m.input.SetValue("agent add build-01 ")
	if matches = m.suggestions(); len(matches) != 3 {
		t.Fatalf("role suggestions after space = %#v", matches)
	}

	m.input.SetValue("agent add build-01 --roles run")
	matches = m.suggestions()
	if len(matches) != 2 || matches[0].Command != "agent add build-01 --roles runner" || matches[1].Command != "agent add build-01 --roles builder,runner" {
		t.Fatalf("filtered role suggestions = %#v", matches)
	}
}

func TestAgentRoleSuggestionsRenderOnlyTheRoleValue(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("agent add build-01")
	m := &model{input: input, width: 80}
	output := m.renderCommandArea()
	if !strings.Contains(output, "builder,runner") {
		t.Fatalf("combined role is not visible:\n%s", output)
	}
	if strings.Count(output, "agent add build-01 --roles") != 0 {
		t.Fatalf("role picker repeats the full command:\n%s", output)
	}
}

func TestSlashAndShowOpenTheWholeCommandCatalog(t *testing.T) {
	for _, value := range []string{"/", "show"} {
		if got := len(matchingCommands(value, 0)); got != len(commandCatalog) {
			t.Fatalf("%q returned %d commands, want %d", value, got, len(commandCatalog))
		}
	}
}

func TestWelcomeIsCompactAndResponsive(t *testing.T) {
	for _, width := range []int{24, 48, 80} {
		m := &model{width: width}
		welcome := m.welcome()
		if !strings.Contains(welcome, "Welcome to URUFLOW") && width >= 48 {
			t.Fatalf("welcome missing title at width %d", width)
		}
		for row, line := range strings.Split(welcome, "\n") {
			if got := utf8Width(line); got > width {
				t.Fatalf("width %d row %d uses %d columns", width, row+1, got)
			}
		}
	}
}

func TestOperationalResponsesHaveBalancedWorkspaceGutters(t *testing.T) {
	output := centerResponse("╭────────╮\n│ status │\n╰────────╯")
	for row, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, " ") {
			t.Fatalf("row %d does not have a left workspace gutter: %q", row+1, line)
		}
	}
}

func TestCommandPaletteNeverExceedsTerminalWidth(t *testing.T) {
	for _, width := range []int{24, 40, 80} {
		for _, value := range []string{"/", "deploy ", "agent add "} {
			input := textinput.New()
			input.Prompt = "› "
			input.SetValue(value)
			m := &model{
				input: input, editor: textarea.New(), viewport: viewport.New(max(20, width-2), 12),
				width: width, height: 24, initialized: true,
			}
			m.resize()
			for row, line := range strings.Split(m.renderCommandArea(), "\n") {
				if got := utf8Width(line); got > width {
					t.Fatalf("width %d input %q row %d uses %d columns", width, value, row+1, got)
				}
			}
		}
	}
}

func TestPaletteWindowFollowsTheSelection(t *testing.T) {
	input := textinput.New()
	input.SetValue("/")
	m := &model{input: input, height: 24, suggestionAt: len(commandCatalog) - 1}
	visible, start := m.visibleSuggestions()
	if len(visible) == 0 || start+len(visible) != len(commandCatalog) {
		t.Fatalf("visible window start=%d len=%d catalog=%d", start, len(visible), len(commandCatalog))
	}
}

func TestStartupDoesNotAutomaticallyRunStatus(t *testing.T) {
	m := &model{}
	_ = m.Init()
	if m.running {
		t.Fatal("startup unexpectedly began a status command")
	}
}

func TestPreviewWorkspace(t *testing.T) {
	if os.Getenv("PREVIEW") == "" {
		t.Skip("set PREVIEW=1")
	}
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "Type a command or / to browse …"
	input.Focus()
	m := &model{
		input: input, editor: textarea.New(), viewport: viewport.New(88, 20),
		width: 90, height: 30, initialized: true,
	}
	m.resize()
	m.transcript = m.welcome()
	m.viewport.SetContent(m.transcript)
	fmt.Println(m.View())

	fmt.Println("\n--- command palette ---")
	m.input.SetValue("/")
	m.resize()
	fmt.Println(m.renderCommandArea())

	fmt.Println("\n--- deploy projects ---")
	m.input.SetValue("deploy ")
	m.completionCache = map[string][]commandSpec{"projects": {
		{Command: "deploy api-prod", Summary: "build_deploy · prod"},
		{Command: "deploy website-prod", Summary: "deploy_only · prod"},
	}}
	m.resize()
	fmt.Println(m.renderCommandArea())
}
