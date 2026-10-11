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
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mustafanass/uruflow/internal/cliui"
	"github.com/mustafanass/uruflow/internal/grammar"
)

const (
	idleCommandAreaRows     = 4
	expandedCommandAreaRows = 5
	maxSuggestionRows       = 6
)

type suggestionRow struct {
	heading string
	item    commandSpec
	index   int
}

func (m *model) append(value string) {
	follow := !m.initialized || m.viewport.AtBottom()
	if !follow {
		m.unseenLines += appendedLineCount(value)
	}
	m.transcript += value
	lines := strings.Split(m.transcript, "\n")
	if len(lines) > maxTranscriptLines {
		lines = lines[len(lines)-keptTranscriptLines:]
		m.transcript = m.paint(cliui.ANSIMuted, "… earlier output omitted …") + "\n" + strings.Join(lines, "\n")
	}
	m.viewport.SetContent(m.transcript)
	if follow {
		m.viewport.GotoBottom()
		m.unseenLines = 0
	}
}

func appendedLineCount(value string) int {
	lines := 0
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(ansi.Strip(line)) != "" {
			lines++
		}
	}
	return lines
}

func centerResponse(value string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		if line != "" {
			lines[index] = " " + line
		}
	}
	return strings.Join(lines, "\n")
}

func (m *model) resize() {
	if !m.initialized {
		return
	}
	if m.paste {
		errorRows := 0
		if m.editorError != "" {
			errorRows = 2
		}
		m.editorRows = max(3, m.height-5-errorRows)
		m.editor.SetWidth(max(8, m.width-4))
		m.editor.SetHeight(m.editorRows)
		return
	}
	inputRows := idleCommandAreaRows
	if m.commandAreaExpanded() {
		inputRows = m.suggestionWindowRows() + expandedCommandAreaRows
	}
	m.viewport.Width = max(20, m.width-2)
	m.viewport.Height = max(4, m.height-inputRows-2)
	m.input.Width = max(10, m.width-8)
	if m.pristine {
		m.transcript = m.welcome()
		m.viewport.SetContent(m.transcript)
		m.viewport.GotoTop()
	}
	m.clearUnseenAtBottom()
}

func (m *model) View() string {
	if !m.initialized {
		return ""
	}
	state, shortState, stateColor := "ready", "ready", cliui.ANSISuccess
	if m.running && m.editorSubmission {
		state, shortState, stateColor = "validating YAML", "validating", cliui.ANSIInformation
		if isVariableEditor(m.active) {
			state, shortState = "saving variables", "saving"
		} else if len(m.active) >= 2 && m.active[0] == "project" && m.active[1] == "create" {
			state, shortState = "creating project", "creating"
		}
	} else if m.running && isVariableEditor(m.active) {
		state, shortState, stateColor = "loading variables", "loading", cliui.ANSIInformation
	} else if m.running && isProjectEditor(m.active) {
		state, shortState, stateColor = "loading project YAML", "loading", cliui.ANSIInformation
	} else if m.running {
		state, shortState = operationState(m.active)
		stateColor = cliui.ANSIInformation
	} else if m.paste {
		if isVariableEditor(m.pending) {
			state, shortState, stateColor = "editing variables", "variables", cliui.ANSIWarning
		} else {
			state, shortState, stateColor = "editing YAML", "YAML", cliui.ANSIWarning
		}
	} else if m.confirm {
		state, shortState, stateColor = "confirmation", "confirm", cliui.ANSIWarning
	}
	brand := cliui.Wordmark(m.color)
	status := "● " + state
	if utf8Width(cliui.PlainWordmark)+utf8Width(status)+2 > m.width {
		status = "● " + shortState
	}
	header := brand + strings.Repeat(" ", max(1, m.width-utf8Width(cliui.PlainWordmark)-utf8Width(status))) + m.paint(stateColor, status)
	rule := m.paint(cliui.ANSIBorder, strings.Repeat("─", max(1, m.width)))
	if m.paste {
		return header + "\n" + rule + "\n" + m.renderEditorArea()
	}
	return header + "\n" + rule + "\n" + m.viewport.View() + "\n" + m.renderCommandArea()
}

func (m *model) renderCommandArea() string {
	suggestions := m.suggestions()
	title := " COMMAND "
	completion, completing := argumentCompletion(m.input.Value())
	if completing {
		title = " " + completion.Trail + " "
	} else if len(suggestions) > 0 || paletteOpen(m.input.Value()) {
		title = " COMMANDS "
	} else if m.confirm {
		title = " CONFIRM "
	}
	lines := []string{m.frameTop(title)}
	content := make([]string, 0, m.suggestionWindowRows())
	commandWidth, summaryWidth := m.suggestionWidths(suggestions, completion, completing)
	for _, row := range m.visibleSuggestionRows(suggestions) {
		if row.heading != "" {
			content = append(content, m.frameRow(m.paint(cliui.ANSIMuted, "  "+row.heading)))
			continue
		}
		selected := row.index == m.suggestionAt
		marker := "  "
		if selected {
			marker = m.paint(cliui.ANSIAccentBold, "› ")
		}
		command := padText(truncateText(suggestionValue(row.item, completion, completing), commandWidth), commandWidth)
		if selected {
			command = m.paint(cliui.ANSISelected, command)
		} else {
			command = m.paint(cliui.ANSIInformation, command)
		}
		summary := truncateText(row.item.Summary, summaryWidth)
		content = append(content, m.frameRow(marker+command+"  "+m.paint(cliui.ANSIMuted, summary)))
	}
	if completing && len(suggestions) == 0 {
		message := "Type a name to continue"
		messageStyle := cliui.ANSIMuted
		if completionError := m.completionErrors[completion.Key]; completionError != "" {
			message = "Could not load " + strings.ToLower(completion.Label) + " · Enter retries · " + completionError
			messageStyle = cliui.ANSIError
		} else if len(completion.Items) > 0 && completion.EmptyTip != "" {
			message = completion.EmptyTip
		} else if len(completion.Request) == 0 && completion.EmptyTip != "" {
			message = completion.EmptyTip
		} else if m.completionLoading[completion.Key] {
			message = "Loading " + strings.ToLower(completion.Label) + " …"
		} else if cached, found := m.completionCache[completion.Key]; found && len(cached) == 0 {
			message = completion.EmptyTip
		} else if found && completion.Query != "" {
			message = "No matching " + strings.ToLower(strings.TrimSuffix(completion.Label, "S"))
		}
		content = append(content, m.frameRow(m.paint(messageStyle, "  "+message)))
	} else if paletteOpen(m.input.Value()) && len(suggestions) == 0 {
		content = append(content, m.frameRow(m.paint(cliui.ANSIMuted, "  No matching commands")))
	}
	if m.commandAreaExpanded() {
		for len(content) < m.suggestionWindowRows() {
			content = append(content, m.frameRow(""))
		}
		lines = append(lines, content...)
		lines = append(lines, m.frameMiddle())
	}
	lines = append(lines, m.frameRow(m.input.View()), m.frameBottom())

	hint := m.fitHint("/ commands · Ctrl+D exit", "/ commands · ^D exit")
	if m.running {
		if isLiveOperation(m.active) {
			if m.unseenLines > 0 {
				hint = m.newOutputHint("Ctrl+C detach")
			} else {
				hint = m.fitHint("Ctrl+C detach", "^C detach")
			}
		} else if m.unseenLines > 0 {
			hint = m.newOutputHint("Working …")
		} else {
			hint = m.fitHint("Working …")
		}
	} else if len(suggestions) > 0 {
		selected := suggestions[min(max(0, m.suggestionAt), len(suggestions)-1)]
		action := "run"
		if selected.NeedsArgs {
			action = "continue"
		}
		hint = m.fitHint(
			"↑↓ navigate · Tab complete · Enter "+action+" · Esc close",
			"↑↓ · Tab complete · Enter "+action+" · Esc",
			"↑↓ · Tab · Enter · Esc",
		)
	} else if completing {
		action := "run"
		if completion.NeedsNext || !completion.AllowRaw {
			action = "continue"
		}
		if m.completionErrors[completion.Key] != "" {
			action = "retry"
		}
		hint = m.fitHint(
			"↑↓ navigate · Tab complete · Enter "+action+" · Esc close",
			"↑↓ · Tab complete · Enter "+action+" · Esc",
			"Type · Enter "+action+" · Esc",
		)
	} else if paletteOpen(m.input.Value()) {
		hint = m.fitHint("Type to filter · Esc close", "Type · Esc close")
	} else if m.confirm {
		hint = m.fitHint("Enter continue · Esc cancel", "Enter · Esc cancel")
	} else if m.unseenLines > 0 {
		hint = m.newOutputHint("")
	}
	lines = append(lines, m.paint(cliui.ANSIMuted, hint))
	return strings.Join(lines, "\n")
}

func operationState(args []string) (string, string) {
	command, err := grammar.Resolve(args)
	if err != nil {
		return "working", "working"
	}
	path := grammar.Path(command)
	if !isLiveOperation(args) {
		return "running · " + path, "running"
	}
	switch path {
	case "events":
		return "following fleet events · Ctrl+C detaches", "live"
	case "container logs":
		return "following container output · Ctrl+C detaches", "live"
	case "project deploy", "project rollback":
		return "following release · Ctrl+C detaches", "live"
	default:
		return "following release output · Ctrl+C detaches", "live"
	}
}

func (m *model) newOutputHint(prefix string) string {
	label := fmt.Sprintf("%d new lines below", m.unseenLines)
	if m.unseenLines == 1 {
		label = "1 new line below"
	}
	compact := fmt.Sprintf("%d new ↓", m.unseenLines)
	if prefix == "" {
		return m.fitHint(label+" · PgDn newer", compact+" · PgDn")
	}
	shortPrefix := "Work"
	if prefix == "Ctrl+C detach" {
		shortPrefix = "^C"
	}
	return m.fitHint(
		prefix+" · "+label+" · PgDn newer",
		prefix+" · "+compact+" · PgDn",
		shortPrefix+" · "+compact+" · PgDn",
	)
}

func (m *model) clearUnseenAtBottom() {
	if m.viewport.AtBottom() {
		m.unseenLines = 0
	}
}

func (m *model) commandAreaExpanded() bool {
	if paletteOpen(m.input.Value()) {
		return true
	}
	if _, completing := argumentCompletion(m.input.Value()); completing {
		return true
	}
	return len(m.suggestions()) > 0
}

func (m *model) suggestionWindowRows() int {
	if m.height <= 0 {
		return maxSuggestionRows
	}
	rows := maxSuggestionRows
	switch {
	case m.height < 20:
		rows = 3
	case m.height < 28:
		rows = 5
	}
	return min(rows, max(1, m.height-11))
}

func (m *model) visibleSuggestionRows(suggestions []commandSpec) []suggestionRow {
	if len(suggestions) == 0 {
		return nil
	}
	rows := make([]suggestionRow, 0, len(suggestions))
	grouped := paletteBrowse(m.input.Value())
	group := ""
	selectedRow := 0
	selected := min(max(0, m.suggestionAt), len(suggestions)-1)
	for index, item := range suggestions {
		if grouped {
			nextGroup := suggestionGroup(item.Command)
			if nextGroup != group {
				rows = append(rows, suggestionRow{heading: nextGroup, index: -1})
				group = nextGroup
			}
		}
		if index == selected {
			selectedRow = len(rows)
		}
		rows = append(rows, suggestionRow{item: item, index: index})
	}
	limit := m.suggestionWindowRows()
	if len(rows) <= limit {
		return rows
	}
	start := max(0, selectedRow-limit+1)
	if start+limit > len(rows) {
		start = len(rows) - limit
	}
	return rows[start : start+limit]
}

func suggestionGroup(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "WORKSPACE"
	}
	switch fields[0] {
	case "status", "events":
		return "FLEET / OBSERVE"
	case "agent":
		return "AGENTS"
	case "container":
		return "CONTAINERS"
	case "project":
		return "PROJECTS"
	case "release":
		return "RELEASES"
	case "registry":
		return "REGISTRY"
	case "alert":
		return "ALERTS"
	default:
		return "WORKSPACE"
	}
}

func (m *model) suggestionWidths(suggestions []commandSpec, completion argumentContext, completing bool) (int, int) {
	longest := 1
	for _, item := range suggestions {
		longest = max(longest, utf8Width(suggestionValue(item, completion, completing)))
	}
	contentWidth := max(1, m.width-4)
	available := max(1, contentWidth-4)
	preferred := min(48, max(10, m.width/3))
	summaryFloor := min(16, max(4, available/3))
	commandWidth := min(longest, max(1, min(preferred, available-summaryFloor)))
	return commandWidth, max(0, available-commandWidth)
}

func (m *model) fitHint(options ...string) string {
	for _, option := range options {
		hint := "  " + option
		if utf8Width(hint) <= m.width {
			return hint
		}
	}
	if len(options) == 0 {
		return ""
	}
	return truncateText("  "+options[len(options)-1], max(1, m.width))
}

func paletteOpen(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "/")
}

func paletteBrowse(value string) bool {
	return strings.TrimSpace(value) == "/"
}

func suggestionValue(item commandSpec, completion argumentContext, completing bool) string {
	if item.Display != "" {
		return item.Display
	}
	if completing && strings.HasPrefix(item.Command, completion.Prefix) {
		return strings.TrimPrefix(item.Command, completion.Prefix)
	}
	return item.Command
}

func (m *model) renderEditorArea() string {
	title := m.editorTitle
	if title == "" {
		title = "YAML"
	}
	hint := m.editorHint
	if hint == "" {
		hint = "Ctrl+S validate and apply · Esc cancel"
	}
	footer := m.fitHint(hint)
	if !isVariableEditor(m.pending) && !isVariableEditor(m.active) {
		footer = m.fitHint(
			hint+" · Enter auto-indent · Tab/Shift+Tab indent",
			"Ctrl+S save · Esc cancel · Tab indent",
			"^S save · Esc cancel",
		)
	}
	lines := []string{m.frameTop(" " + title + " ")}
	if m.editorError != "" {
		lines = append(lines, m.frameRow(m.paint(cliui.ANSIError, "✘ "+m.editorError)), m.frameMiddle())
	}
	for _, line := range strings.Split(m.editor.View(), "\n") {
		lines = append(lines, m.frameRow(line))
	}
	lines = append(lines, m.frameBottom(), m.paint(cliui.ANSIMuted, footer))
	return strings.Join(lines, "\n")
}

func (m *model) frameTop(title string) string {
	inner := max(1, m.width-2)
	label := truncateText(title, inner)
	return m.paint(cliui.ANSIBorder, "╭") + m.paint(cliui.ANSIAccentBold, label) + m.paint(cliui.ANSIBorder, strings.Repeat("─", max(0, inner-utf8Width(label)))+"╮")
}

func (m *model) frameMiddle() string {
	return m.paint(cliui.ANSIBorder, "├"+strings.Repeat("─", max(1, m.width-2))+"┤")
}

func (m *model) frameBottom() string {
	return m.paint(cliui.ANSIBorder, "╰"+strings.Repeat("─", max(1, m.width-2))+"╯")
}

func (m *model) frameRow(value string) string {
	contentWidth := max(1, m.width-4)
	value = truncateText(value, contentWidth)
	return m.paint(cliui.ANSIBorder, "│") + " " + value + strings.Repeat(" ", max(0, contentWidth-utf8Width(value))) + " " + m.paint(cliui.ANSIBorder, "│")
}

func (m *model) welcome() string {
	width := min(66, max(20, m.width-6))
	inner := width - 2
	row := func(value string) string {
		value = truncateText(value, inner-4)
		return m.paint(cliui.ANSIBorder, "│") + "  " + value + strings.Repeat(" ", max(0, inner-4-utf8Width(value))) + "  " + m.paint(cliui.ANSIBorder, "│")
	}
	lines := []string{
		m.paint(cliui.ANSIBorder, "╭"+strings.Repeat("─", inner)+"╮"),
		row(""),
		row(cliui.Wordmark(m.color)),
		row(m.paint(cliui.ANSIMuted, "Self-hosted deployment control plane")),
		row(m.paint(cliui.ANSIMuted, "One workspace for your whole fleet.")),
		row(""),
		row(m.paint(cliui.ANSIAccent, "status") + "          Fleet health and active work"),
		row(m.paint(cliui.ANSIAccent, "events") + "          Follow live fleet activity"),
		row(m.paint(cliui.ANSIAccent, "project deploy") + "  Start and follow a release"),
		row(m.paint(cliui.ANSIAccent, "/") + "               Browse every command"),
		row(""),
		m.paint(cliui.ANSIBorder, "╰"+strings.Repeat("─", inner)+"╯"),
	}
	indent := strings.Repeat(" ", max(0, (m.width-2-width)/2))
	for index := range lines {
		lines[index] = indent + lines[index]
	}
	vertical := strings.Repeat("\n", max(0, (m.viewport.Height-len(lines))/2))
	return vertical + strings.Join(lines, "\n")
}

func (m *model) streamWelcome() string {
	width := min(66, max(20, m.width-4))
	inner := width - 2
	row := func(value string) string {
		value = truncateText(value, inner-4)
		return m.paint(cliui.ANSIBorder, "│") + "  " + value + strings.Repeat(" ", max(0, inner-4-utf8Width(value))) + "  " + m.paint(cliui.ANSIBorder, "│")
	}
	return strings.Join([]string{
		m.paint(cliui.ANSIBorder, "╭"+strings.Repeat("─", inner)+"╮"),
		row(m.paint(cliui.ANSIAccentBold, "●  LIVE FLEET ACTIVITY")),
		row("Releases · build output · state changes · alerts"),
		row(m.paint(cliui.ANSIMuted, "Streaming from now · Ctrl+C detaches")),
		m.paint(cliui.ANSIBorder, "╰"+strings.Repeat("─", inner)+"╯"),
		"",
	}, "\n")
}

func (m *model) paint(code, value string) string {
	return cliui.Paint(m.color, code, value)
}

func utf8Width(value string) int { return lipgloss.Width(value) }

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8Width(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "…")
}

func padText(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-utf8Width(value)))
}
