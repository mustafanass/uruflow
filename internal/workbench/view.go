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

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mustafanass/uruflow/internal/cliui"
)

func (m *model) append(value string) {
	follow := !m.initialized || m.viewport.AtBottom()
	m.transcript += value
	lines := strings.Split(m.transcript, "\n")
	if len(lines) > maxTranscriptLines {
		lines = lines[len(lines)-keptTranscriptLines:]
		m.transcript = m.paint(cliui.ANSIMuted, "… earlier output omitted …") + "\n" + strings.Join(lines, "\n")
	}
	m.viewport.SetContent(m.transcript)
	if follow {
		m.viewport.GotoBottom()
	}
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
	inputRows := 4
	if visible, _ := m.visibleSuggestions(); len(visible) > 0 {
		inputRows += len(visible) + 1
	} else if _, ok := argumentCompletion(m.input.Value()); ok {
		inputRows += 2
	}
	m.viewport.Width = max(20, m.width-2)
	m.viewport.Height = max(4, m.height-inputRows-2)
	m.input.Width = max(10, m.width-8)
	if m.pristine {
		m.transcript = m.welcome()
		m.viewport.SetContent(m.transcript)
		m.viewport.GotoTop()
	}
}

func (m *model) View() string {
	if !m.initialized {
		return ""
	}
	state, shortState, stateColor := "ready", "ready", cliui.ANSISuccess
	if m.running && m.editorSubmission {
		state, shortState, stateColor = "validating YAML", "validating", cliui.ANSIInformation
		if len(m.active) >= 2 && m.active[0] == "project" && m.active[1] == "create" {
			state, shortState = "creating project", "creating"
		}
	} else if m.running {
		state, shortState = "streaming · Ctrl+C detaches", "live"
	} else if m.paste {
		state, shortState, stateColor = "editing YAML", "YAML", cliui.ANSIWarning
	} else if m.secret {
		state, shortState, stateColor = "secure input", "secret", cliui.ANSIWarning
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
		return header + "\n" + rule + "\n" + m.renderYAMLArea()
	}
	return header + "\n" + rule + "\n" + m.viewport.View() + "\n" + m.renderCommandArea()
}

func (m *model) renderCommandArea() string {
	visible, start := m.visibleSuggestions()
	title := " COMMAND "
	completion, completing := argumentCompletion(m.input.Value())
	if completing {
		title = " " + completion.Trail + " "
	} else if len(visible) > 0 {
		title = " COMMANDS "
	} else if m.secret {
		title = " SECURE VALUE "
	} else if m.confirm {
		title = " CONFIRM "
	}
	lines := []string{m.frameTop(title)}
	commandWidth := 0
	for _, item := range visible {
		commandWidth = max(commandWidth, utf8Width(suggestionValue(item, completion, completing)))
	}
	commandWidth = min(commandWidth, 24)
	for index, item := range visible {
		selected := start+index == m.suggestionAt
		marker := "  "
		if selected {
			marker = m.paint(cliui.ANSIAccentBold, "› ")
		}
		command := padText(suggestionValue(item, completion, completing), commandWidth)
		if selected {
			command = m.paint(cliui.ANSISelected, command)
		} else {
			command = m.paint(cliui.ANSIInformation, command)
		}
		lines = append(lines, m.frameRow(marker+command+"  "+m.paint(cliui.ANSIMuted, item.Summary)))
	}
	if completing && len(visible) == 0 {
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
		lines = append(lines, m.frameRow(m.paint(messageStyle, "  "+message)))
	}
	if len(visible) > 0 || completing {
		lines = append(lines, m.frameMiddle())
	}
	lines = append(lines, m.frameRow(m.input.View()), m.frameBottom())

	hint := "  ↑↓ history · / commands · PgUp/PgDn scroll · Ctrl+D exit"
	if len(visible) > 0 {
		selected := visible[min(max(0, m.suggestionAt-start), len(visible)-1)]
		action := "run"
		if selected.NeedsArgs {
			action = "continue"
		}
		hint = "  ↑↓ select · Tab complete · Enter " + action + " · Esc close"
	} else if completing {
		action := "run"
		if completion.NeedsNext || !completion.AllowRaw {
			action = "continue"
		}
		if m.completionErrors[completion.Key] != "" {
			action = "retry"
		}
		hint = "  Type to filter · ↑↓ select · Enter " + action
	} else if m.secret || m.confirm {
		hint = "  Enter continue · Esc cancel"
	}
	if m.width < utf8Width(hint) {
		hint = "  / commands · ^D exit"
	}
	lines = append(lines, m.paint(cliui.ANSIMuted, hint))
	return strings.Join(lines, "\n")
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

func (m *model) renderYAMLArea() string {
	title := m.editorTitle
	if title == "" {
		title = "YAML"
	}
	hint := m.editorHint
	if hint == "" {
		hint = "Ctrl+S validate and apply · Esc cancel"
	}
	footer := "  " + hint + " · Enter smart indent · Tab/Shift+Tab indent"
	if utf8Width(footer) > m.width {
		footer = "  Ctrl+S save · Esc cancel · Enter/Tab smart indent"
	}
	footer = truncateText(footer, max(1, m.width))
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
