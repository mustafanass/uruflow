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
