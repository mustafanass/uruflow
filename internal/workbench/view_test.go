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
	tea "github.com/charmbracelet/bubbletea"
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
	for _, width := range []int{24, 40, 80, 100, 120, 160} {
		for _, value := range []string{"/", "/project", "project deploy ", "agent add "} {
			input := textinput.New()
			input.Prompt = "› "
			input.SetValue(value)
			for _, color := range []bool{false, true} {
				m := &model{
					input: input, editor: textarea.New(), viewport: viewport.New(max(20, width-2), 12),
					width: width, height: 24, initialized: true, color: color,
				}
				m.resize()
				for row, line := range strings.Split(m.renderCommandArea(), "\n") {
					if got := utf8Width(line); got > width {
						t.Fatalf("width %d color %v input %q row %d uses %d columns", width, color, value, row+1, got)
					}
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

func TestSuggestionAreaUsesABoundedStableWindow(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	m := &model{
		input: input, editor: textarea.New(), viewport: viewport.New(78, 18),
		width: 80, height: 24, initialized: true,
	}
	m.resize()
	idleHeight := m.viewport.Height
	if rows := len(strings.Split(m.renderCommandArea(), "\n")); rows != idleCommandAreaRows {
		t.Fatalf("idle command area uses %d rows", rows)
	}

	for _, value := range []string{"/", "/project", "/project dep", "/nothing-matches"} {
		m.input.SetValue(value)
		m.resize()
		if m.viewport.Height != 12 {
			t.Fatalf("input %q changed viewport height to %d", value, m.viewport.Height)
		}
		if rows := len(strings.Split(m.renderCommandArea(), "\n")); rows != m.suggestionWindowRows()+expandedCommandAreaRows {
			t.Fatalf("input %q command area uses %d rows", value, rows)
		}
		if value == "/nothing-matches" && !strings.Contains(m.renderCommandArea(), "Esc close") {
			t.Fatalf("empty palette does not explain how to close it")
		}
	}
	if movement := idleHeight - m.viewport.Height; movement != m.suggestionWindowRows()+1 || movement > maxSuggestionRows+1 {
		t.Fatalf("opening movement = %d rows", movement)
	}
	m.input.SetValue("")
	m.resize()
	if m.viewport.Height != idleHeight {
		t.Fatalf("closing restored viewport height %d, want %d", m.viewport.Height, idleHeight)
	}
}

func TestGroupedPaletteKeepsEverySelectionVisible(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("/")
	m := &model{input: input, width: 100, height: 30}
	for index := range commandCatalog {
		m.suggestionAt = index
		rows := m.visibleSuggestionRows(commandCatalog)
		if len(rows) > m.suggestionWindowRows() {
			t.Fatalf("selection %d rendered %d rows", index, len(rows))
		}
		visible := false
		for _, row := range rows {
			visible = visible || row.heading == "" && row.index == index
		}
		if !visible {
			t.Fatalf("selection %d is outside %#v", index, rows)
		}
	}
}

func TestPaletteGroupsComeFromCommandHierarchyAndFilteringStillWorks(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("/")
	m := &model{input: input, width: 100, height: 30}
	output := m.renderCommandArea()
	for _, group := range []string{"FLEET / OBSERVE", "AGENTS"} {
		if !strings.Contains(output, group) {
			t.Fatalf("palette missing group %q:\n%s", group, output)
		}
	}
	m.input.SetValue("/deploy")
	m.suggestionAt = 0
	output = m.renderCommandArea()
	if !strings.Contains(output, "project deploy") || strings.Contains(output, "FLEET / OBSERVE") {
		t.Fatalf("filtered palette is not a natural command list:\n%s", output)
	}
}

func TestLongSuggestionsTruncateWithoutBreakingLayout(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("project deploy ")
	completion, _ := argumentCompletion(input.Value())
	item := commandSpec{
		Command: "project deploy " + strings.Repeat("界", 40),
		Summary: strings.Repeat("very long deployment summary ", 10),
	}
	m := &model{
		input: input, width: 40, height: 24, color: true,
		completionCache: map[string][]commandSpec{completion.Key: {item}},
	}
	output := m.renderCommandArea()
	if !strings.Contains(output, "…") {
		t.Fatalf("long suggestion was not truncated:\n%s", output)
	}
	for row, line := range strings.Split(output, "\n") {
		if got := utf8Width(line); got > m.width {
			t.Fatalf("row %d uses %d columns:\n%s", row+1, got, output)
		}
	}
}

func TestSuggestionColumnsUseAdditionalWideTerminalSpace(t *testing.T) {
	items := []commandSpec{{Command: strings.Repeat("command", 12), Summary: strings.Repeat("summary ", 20)}}
	completion := argumentContext{}
	narrow := &model{width: 80}
	wide := &model{width: 160}
	narrowCommand, narrowSummary := narrow.suggestionWidths(items, completion, false)
	wideCommand, wideSummary := wide.suggestionWidths(items, completion, false)
	if wideCommand <= narrowCommand || wideSummary <= narrowSummary {
		t.Fatalf("columns did not grow: narrow=%d/%d wide=%d/%d", narrowCommand, narrowSummary, wideCommand, wideSummary)
	}
}

func TestCommandAreaWithoutColorHasNoANSI(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("/")
	m := &model{input: input, width: 80, height: 24, color: false}
	if output := m.renderCommandArea(); strings.Contains(output, "\x1b[") {
		t.Fatalf("NO_COLOR command area contains ANSI: %q", output)
	}
}

func TestNarrowWorkspaceRemainsWithinTheTerminal(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("/")
	m := &model{
		input: input, editor: textarea.New(), viewport: viewport.New(22, 12),
		width: 24, height: 24, initialized: true,
	}
	m.resize()
	output := m.View()
	lines := strings.Split(output, "\n")
	if len(lines) != m.height {
		t.Fatalf("narrow workspace uses %d rows, want %d", len(lines), m.height)
	}
	for row, line := range lines {
		if got := utf8Width(line); got > m.width {
			t.Fatalf("row %d uses %d columns", row+1, got)
		}
	}
}

func TestAppendingOutputDoesNotMoveAnOlderTranscriptView(t *testing.T) {
	view := viewport.New(80, 4)
	transcript := strings.Join([]string{"one", "two", "three", "four", "five", "six"}, "\n")
	view.SetContent(transcript)
	view.GotoTop()
	input := textinput.New()
	input.Prompt = "› "
	m := &model{
		input: input, viewport: view, transcript: transcript, initialized: true,
		width: 80, height: 24, running: true, active: []string{"events"}, cancel: func() {},
	}
	m.append("\nseven")
	if m.viewport.YOffset != 0 {
		t.Fatalf("append moved viewport to offset %d", m.viewport.YOffset)
	}
	if m.unseenLines != 1 {
		t.Fatalf("unseen lines = %d, want 1", m.unseenLines)
	}
	output := m.renderCommandArea()
	if !strings.Contains(output, "1 new line below") || !strings.Contains(output, "Ctrl+C detach") {
		t.Fatalf("new output is not visible in the running workspace:\n%s", output)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.unseenLines != 0 || !m.viewport.AtBottom() {
		t.Fatalf("PgDn left offset=%d unseen=%d", m.viewport.YOffset, m.unseenLines)
	}
}

func TestNewOutputHintFitsNarrowTerminals(t *testing.T) {
	m := &model{width: 24, unseenLines: 27}
	for _, prefix := range []string{"", "Ctrl+C detach", "Working …"} {
		hint := m.newOutputHint(prefix)
		if got := utf8Width(hint); got > m.width {
			t.Fatalf("prefix %q uses %d columns: %q", prefix, got, hint)
		}
		if !strings.Contains(hint, "27 new") {
			t.Fatalf("prefix %q lost the new-output count: %q", prefix, hint)
		}
	}
}

func TestAppendedLineCountIgnoresBlankAndANSIOnlyRows(t *testing.T) {
	value := "\x1b[32mrelease started\x1b[0m\n\n\x1b[0m\nbuild queued\n"
	if got := appendedLineCount(value); got != 2 {
		t.Fatalf("appended line count = %d, want 2", got)
	}
}
