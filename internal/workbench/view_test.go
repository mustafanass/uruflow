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

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestProjectEditorUsesTheFullWorkspaceBody(t *testing.T) {
	input := textinput.New()
	editor := textarea.New()
	editor.SetValue("project:\n  name: api\n")
	editor.Focus()
	m := &model{
		input: input, editor: editor, viewport: viewport.New(88, 20),
		width: 90, height: 30, initialized: true, paste: true, editorTitle: "NEW PROJECT",
		transcript: "ordinary response must be hidden while editing",
	}
	m.resize()
	output := m.View()
	if m.editor.Height() != 25 || m.editor.Width() < 80 {
		t.Fatalf("editor size = %dx%d", m.editor.Width(), m.editor.Height())
	}
	if strings.Contains(output, "ordinary response must be hidden") || !strings.Contains(output, "NEW PROJECT") {
		t.Fatalf("editor did not replace the response body:\n%s", output)
	}
	for row, line := range strings.Split(output, "\n") {
		if got := utf8Width(line); got > m.width {
			t.Fatalf("row %d uses %d columns at width %d", row+1, got, m.width)
		}
	}
}

func TestProjectEditorRendersSaveErrorsInsideTheWorkspace(t *testing.T) {
	input := textinput.New()
	editor := textarea.New()
	editor.SetValue("project:\n  name: api\n")
	editor.Focus()
	m := &model{
		input: input, editor: editor, viewport: viewport.New(88, 20),
		width: 90, height: 30, initialized: true, paste: true, editorTitle: "NEW PROJECT",
		editorError: "unknown agent runner-01", transcript: "response page error must stay hidden",
	}
	m.resize()
	output := m.View()
	if !strings.Contains(output, "unknown agent runner-01") || strings.Contains(output, "response page error must stay hidden") {
		t.Fatalf("save error did not stay with the editor:\n%s", output)
	}
}

func TestWelcomeIsCompactAndResponsive(t *testing.T) {
	for _, width := range []int{24, 48, 80} {
		m := &model{width: width}
		welcome := m.welcome()
		if !strings.Contains(welcome, "uruflow") && width >= 48 {
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
		for _, value := range []string{"/", "project deploy ", "agent add "} {
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
