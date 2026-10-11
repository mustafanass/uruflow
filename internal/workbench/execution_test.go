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
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/ops"
)

func TestCommandLineSupportsQuotes(t *testing.T) {
	args, err := split(`project show "api prod"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || args[2] != "api prod" {
		t.Fatalf("args = %#v", args)
	}
}

func TestEventsUseAFocusedStreamingPage(t *testing.T) {
	for _, width := range []int{24, 60, 100} {
		m := &model{width: width}
		for row, line := range strings.Split(m.streamWelcome(), "\n") {
			if got := utf8Width(line); got > width {
				t.Fatalf("width %d row %d uses %d columns", width, row+1, got)
			}
		}
	}
}

func TestEventDetachShowsTheResumeCursor(t *testing.T) {
	input := textinput.New()
	view := viewport.New(80, 12)
	cancelled := false
	m := &model{input: input, viewport: view, running: true, active: []string{"events"},
		cancel: func() { cancelled = true }, events: make(chan streamMsg, 1)}
	_, _ = m.Update(streamMsg{event: &ops.Event{Type: ops.EventMessage, Sequence: 42, Message: "release started"}})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !cancelled || !strings.Contains(m.transcript, "resume with events --after 42") {
		t.Fatalf("cancelled=%v transcript=%q", cancelled, m.transcript)
	}
}

func TestStartupDoesNotAutomaticallyRunStatus(t *testing.T) {
	m := &model{}
	_ = m.Init()
	if m.running {
		t.Fatal("startup unexpectedly began a status command")
	}
}

func TestOperationStateDistinguishesCommandsFromLiveOutput(t *testing.T) {
	for _, test := range []struct {
		args  []string
		state string
		live  bool
	}{
		{[]string{"status"}, "running · status", false},
		{[]string{"events"}, "following fleet events · Ctrl+C detaches", true},
		{[]string{"project", "deploy", "api-prod"}, "following release · Ctrl+C detaches", true},
		{[]string{"project", "deploy", "api-prod", "--no-follow"}, "running · project deploy", false},
		{[]string{"release", "logs", "release-01"}, "running · release logs", false},
		{[]string{"release", "logs", "release-01", "--follow"}, "following release output · Ctrl+C detaches", true},
		{[]string{"container", "logs", "runner-01", "container-01", "--follow"}, "following container output · Ctrl+C detaches", true},
	} {
		state, _ := operationState(test.args)
		if state != test.state || isLiveOperation(test.args) != test.live {
			t.Errorf("%v state=%q live=%v", test.args, state, isLiveOperation(test.args))
		}
	}
}

func TestProjectEditorLoadingHasAnAccurateWorkspaceState(t *testing.T) {
	input := textinput.New()
	view := viewport.New(78, 18)
	m := &model{
		input: input, viewport: view, width: 80, height: 24, initialized: true,
		running: true, active: []string{"project", "edit", "api-prod"},
	}
	m.resize()
	output := m.View()
	if !strings.Contains(output, "loading project YAML") || strings.Contains(output, "streaming") {
		t.Fatalf("project editor loading state is inaccurate:\n%s", output)
	}
}

func TestCtrlCUsesAccurateFiniteAndDurableMessages(t *testing.T) {
	for _, test := range []struct {
		args     []string
		expected string
	}{
		{[]string{"status"}, "stopped waiting for command output"},
		{[]string{"project", "deploy", "api-prod"}, "server-side release continues"},
	} {
		input := textinput.New()
		view := viewport.New(80, 12)
		cancelled := false
		m := &model{
			input: input, viewport: view, running: true, active: test.args,
			cancel: func() { cancelled = true }, events: make(chan streamMsg, 1),
		}
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if !cancelled || !strings.Contains(m.transcript, test.expected) {
			t.Errorf("%v cancelled=%v transcript=%q", test.args, cancelled, m.transcript)
		}
	}
}
