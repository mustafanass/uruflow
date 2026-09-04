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

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/cliui"
	"github.com/mustafanass/uruflow/internal/grammar"
)

func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height, m.initialized = msg.Width, msg.Height, true
		m.resize()
		return m, nil
	case streamMsg:
		if msg.event != nil {
			if len(m.active) > 0 && m.active[0] == "events" && msg.event.Sequence > m.activityCursor {
				m.activityCursor = msg.event.Sequence
			}
			renderer := cliui.New(nil, m.color)
			renderer.Width = max(20, m.width-4)
			m.append(centerResponse(renderer.RenderString(*msg.event)) + "\n")
		}
		if msg.err != nil {
			if m.editorSubmission {
				m.editorError = cliui.SafeText(msg.err.Error())
			} else {
				m.append(m.paint(cliui.ANSIError, "✘ "+cliui.SafeText(msg.err.Error())) + "\n")
			}
		}
		if msg.done || msg.err != nil {
			m.running = false
			m.cancel = nil
			m.completionCache = make(map[string][]commandSpec)
			m.completionLoading = make(map[string]bool)
			m.completionErrors = make(map[string]string)
			if msg.err != nil && m.editorSubmission {
				m.paste = true
				m.pending = append([]string{}, m.active...)
				m.editorSubmission = false
				m.editor.Focus()
				m.input.Blur()
				m.active = nil
				m.resize()
				return m, textarea.Blink
			}
			m.active = nil
			m.editorSubmission = false
			m.editorTitle, m.editorHint, m.editorError = "", "", ""
			m.editor.SetValue("")
			m.input.Focus()
			return m, textinput.Blink
		}
		return m, m.waitStream()
	case completionMsg:
		if m.completionCache == nil {
			m.completionCache = make(map[string][]commandSpec)
		}
		if m.completionLoading == nil {
			m.completionLoading = make(map[string]bool)
		}
		if m.completionErrors == nil {
			m.completionErrors = make(map[string]string)
		}
		delete(m.completionLoading, msg.key)
		if msg.err == nil {
			m.completionCache[msg.key] = msg.items
			delete(m.completionErrors, msg.key)
		} else {
			m.completionErrors[msg.key] = cliui.SafeText(msg.err.Error())
		}
		m.suggestionAt = 0
		m.resize()
		return m, nil
	case projectEditorMsg:
		m.running = false
		m.active = nil
		if msg.err != nil {
			m.input.Focus()
			m.append(m.paint(cliui.ANSIError, "✘ "+cliui.SafeText(msg.err.Error())) + "\n")
			return m, textinput.Blink
		}
		m.pending, m.paste = append([]string{}, msg.args...), true
		m.prepareEditor(msg.args)
		m.editor.SetValue(msg.content)
		m.editor.CursorStart()
		m.editor.Focus()
		m.resize()
		return m, textarea.Blink
	case variableEditorMsg:
		m.running = false
		m.active = nil
		if msg.err != nil {
			m.input.Focus()
			m.append(m.paint(cliui.ANSIError, "✘ "+cliui.SafeText(msg.err.Error())) + "\n")
			return m, textinput.Blink
		}
		m.pending, m.paste = append([]string{}, msg.args...), true
		m.prepareEditor(msg.args)
		m.editor.SetValue(msg.content)
		m.editor.CursorEnd()
		m.editor.Focus()
		m.resize()
		return m, textarea.Blink
	case tea.KeyMsg:
		if m.paste {
			return m.updatePaste(msg)
		}
		if m.confirm {
			return m.updateConfirm(msg)
		}
		switch msg.String() {
		case "ctrl+d":
			if !m.running {
				return m, tea.Quit
			}
		case "ctrl+c":
			if m.running && m.cancel != nil {
				m.cancel()
				message := "▲ detached from live output"
				if isDurableOperation(m.active) {
					message += "; the server-side release continues"
				} else if len(m.active) > 0 && m.active[0] == "events" && m.activityCursor > 0 {
					message += fmt.Sprintf("; resume with events --after %d", m.activityCursor)
				}
				m.append(m.paint(cliui.ANSIWarning, message) + "\n")
				return m, nil
			}
			m.input.SetValue("")
			m.suggestionAt = 0
			m.resize()
			return m, nil
		case "ctrl+l":
			m.transcript = ""
			m.pristine = false
			m.viewport.SetContent("")
			m.input.SetValue("")
			m.suggestionAt = 0
			m.resize()
			return m, nil
		case "esc":
			if m.input.Value() != "" {
				m.input.SetValue("")
				m.suggestionAt = 0
				m.resize()
				return m, nil
			}
		case "tab":
			if m.completeSuggestion() {
				return m, m.requestArgumentCompletion()
			}
		case "shift+tab":
			if suggestions := m.suggestions(); len(suggestions) > 0 {
				m.suggestionAt = (m.suggestionAt - 1 + len(suggestions)) % len(suggestions)
				return m, nil
			}
		case "up":
			if suggestions := m.suggestions(); len(suggestions) > 0 {
				m.suggestionAt = (m.suggestionAt - 1 + len(suggestions)) % len(suggestions)
				return m, nil
			}
			if m.running {
				break
			}
			m.previousHistory()
			m.resize()
			return m, nil
		case "down":
			if suggestions := m.suggestions(); len(suggestions) > 0 {
				m.suggestionAt = (m.suggestionAt + 1) % len(suggestions)
				return m, nil
			}
			if m.running {
				break
			}
			m.nextHistory()
			m.resize()
			return m, nil
		case "enter":
			if m.running {
				return m, nil
			}
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}
			if suggestions := m.suggestions(); len(suggestions) > 0 {
				selected := suggestions[min(m.suggestionAt, len(suggestions)-1)]
				if selected.NeedsArgs {
					m.setCompletion(selected)
					return m, m.requestArgumentCompletion()
				}
				line = selected.Command
			} else if completion, ok := argumentCompletion(m.input.Value()); ok && !completion.AllowRaw {
				if m.completionErrors[completion.Key] != "" {
					return m, m.retryArgumentCompletion()
				}
				return m, m.requestArgumentCompletion()
			}
			line = strings.TrimSpace(strings.TrimPrefix(line, "/"))
			m.input.SetValue("")
			m.suggestionAt = 0
			m.resize()
			args, err := split(line)
			if err != nil {
				m.append(m.paint(cliui.ANSIError, "✘ "+cliui.SafeText(err.Error())) + "\n")
				return m, nil
			}
			command, err := grammar.Resolve(args)
			if err != nil {
				m.append(m.paint(cliui.ANSIError, "✘ "+cliui.SafeText(err.Error())) + "\n")
				return m, nil
			}
			m.remember(line)
			m.pristine = false
			if command.Action == grammar.ActionExit {
				return m, tea.Quit
			}
			if command.Action == grammar.ActionClear {
				m.transcript = ""
				m.viewport.SetContent("")
				return m, nil
			}
			if command.Focused {
				m.activityCursor = 0
				m.transcript = m.streamWelcome()
				m.viewport.SetContent(m.transcript)
				m.viewport.GotoTop()
			}
			m.append(m.paint(cliui.ANSIAccent, "› "+line) + "\n\n")
			if command.Input == grammar.InputVariables {
				m.running = true
				m.active = append([]string{}, args...)
				m.input.Blur()
				return m, m.loadVariableEditor(args)
			}
			if isProjectEditor(args) {
				m.running = true
				m.active = append([]string{}, args...)
				m.input.Blur()
				return m, m.loadProjectEditor(args)
			}
			if wantsPaste(command, args) {
				m.pending, m.paste = args, true
				m.prepareEditor(args)
				m.editor.Focus()
				m.resize()
				return m, textarea.Blink
			}
			if command.Confirm {
				m.pending, m.confirm = args, true
				m.input.SetValue("")
				m.input.Prompt = "Confirm [y/N] › "
				m.input.Placeholder = strings.Join(args, " ")
				m.resize()
				return m, textinput.Blink
			}
			return m, m.start(args, "")
		}
	}
	if m.paste {
		var command tea.Cmd
		m.editor, command = m.editor.Update(message)
		commands = append(commands, command)
	} else if !m.running {
		var command tea.Cmd
		before := m.input.Value()
		m.input, command = m.input.Update(message)
		if before != m.input.Value() {
			m.suggestionAt = 0
			m.resize()
			commands = append(commands, m.requestArgumentCompletion())
		}
		commands = append(commands, command)
	}
	var viewportCommand tea.Cmd
	m.viewport, viewportCommand = m.viewport.Update(message)
	commands = append(commands, viewportCommand)
	return m, tea.Batch(commands...)
}
