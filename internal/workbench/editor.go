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
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/cliui"
	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/ops"
	"gopkg.in/yaml.v3"
)

const yamlIndentWidth = 2

func (m *model) resolveEditor(name string) tea.Cmd {
	return func() tea.Msg {
		path := ""
		err := m.client.Execute(context.Background(), []string{"project", "path", name}, "", func(event ops.Event) error {
			if event.Title == "project file" {
				path = event.Message
			}
			return nil
		})
		if err == nil && path == "" {
			err = errors.New("server did not return a project file")
		}
		return editPathMsg{path: path, err: err}
	}
}

func (m *model) loadVariableEditor(args []string) tea.Cmd {
	return func() tea.Msg {
		content := ""
		request := []string{"project", "variables-source", args[2]}
		err := m.client.Execute(context.Background(), request, "", func(event ops.Event) error {
			if event.Title == "project variables editor" {
				content = event.Message
			}
			return nil
		})
		if err == nil && content == "" {
			err = errors.New("server did not return project variables")
		}
		return variableEditorMsg{args: append([]string{}, args...), content: content, err: err}
	}
}

func (m *model) updatePaste(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		variableEditor := isVariableEditor(m.pending)
		m.paste, m.pending = false, nil
		m.editorSubmission = false
		m.editorTitle, m.editorHint, m.editorError = "", "", ""
		m.editor.SetValue("")
		m.editor.Blur()
		m.input.Focus()
		message := "▲ YAML input cancelled"
		if variableEditor {
			message = "▲ variable editor cancelled"
		}
		m.append(m.paint(cliui.ANSIWarning, message) + "\n")
		m.resize()
		return m, textinput.Blink
	case "ctrl+s":
		content := m.editor.Value()
		args := append([]string{}, m.pending...)
		content = formatEditorYAML(args, content)
		m.editor.SetValue(content)
		m.editorError = ""
		m.paste, m.pending = false, nil
		m.editorSubmission = true
		m.editor.Blur()
		m.append(m.paint(cliui.ANSIInformation, "• "+editorSubmissionNotice(args)) + "\n")
		m.resize()
		return m, m.start(args, content)
	case "enter":
		if !isVariableEditor(m.pending) {
			m.insertYAMLNewline()
			return m, textarea.Blink
		}
	case "tab":
		if !isVariableEditor(m.pending) {
			m.insertYAMLIndent()
			return m, textarea.Blink
		}
	case "shift+tab":
		if !isVariableEditor(m.pending) {
			m.removeYAMLIndent()
			return m, textarea.Blink
		}
	case "backspace":
		if !isVariableEditor(m.pending) && m.onYAMLIndentOnly() {
			m.removeYAMLIndent()
			return m, textarea.Blink
		}
	}
	if key.Paste && !isVariableEditor(m.pending) {
		key.Runes = []rune(strings.ReplaceAll(string(key.Runes), "\t", strings.Repeat(" ", yamlIndentWidth)))
	}
	var command tea.Cmd
	m.editor, command = m.editor.Update(key)
	return m, command
}

func (m *model) insertYAMLNewline() {
	lines := strings.Split(m.editor.Value(), "\n")
	row, column := editorCursor(m.editor)
	if row < 0 || row >= len(lines) {
		m.editor.InsertString("\n")
		return
	}
	runes := []rune(lines[row])
	column = min(max(0, column), len(runes))
	before := string(runes[:column])
	indent := leadingSpaces(before)
	trimmed := strings.TrimSpace(before)
	next := strings.Repeat(" ", indent)
	switch {
	case strings.HasSuffix(trimmed, ":"):
		next += strings.Repeat(" ", yamlIndentWidth)
	case strings.HasPrefix(trimmed, "- ") && trimmed != "-":
		next += "- "
	}
	m.editor.InsertString("\n" + next)
}

func (m *model) insertYAMLIndent() {
	_, column := editorCursor(m.editor)
	spaces := yamlIndentWidth - column%yamlIndentWidth
	m.editor.InsertString(strings.Repeat(" ", spaces))
}

func (m *model) removeYAMLIndent() {
	value := m.editor.Value()
	lines := strings.Split(value, "\n")
	row, column := editorCursor(m.editor)
	if row < 0 || row >= len(lines) {
		return
	}
	runes := []rune(lines[row])
	indent := leadingSpaces(lines[row])
	remove := indent % yamlIndentWidth
	if remove == 0 {
		remove = min(yamlIndentWidth, indent)
	}
	if remove == 0 {
		return
	}
	lines[row] = string(runes[remove:])
	m.editor.SetValue(strings.Join(lines, "\n"))
	moveEditorCursor(&m.editor, row, max(0, column-remove))
}

func (m *model) onYAMLIndentOnly() bool {
	lines := strings.Split(m.editor.Value(), "\n")
	row, column := editorCursor(m.editor)
	if row < 0 || row >= len(lines) || column == 0 {
		return false
	}
	runes := []rune(lines[row])
	column = min(column, len(runes))
	return strings.TrimSpace(string(runes[:column])) == ""
}

func editorCursor(editor textarea.Model) (int, int) {
	info := editor.LineInfo()
	return editor.Line(), info.StartColumn + info.ColumnOffset
}

func moveEditorCursor(editor *textarea.Model, row, column int) {
	editor.CursorStart()
	for editor.Line() > row {
		editor.CursorUp()
	}
	for editor.Line() < row {
		editor.CursorDown()
	}
	editor.SetCursor(column)
}

func leadingSpaces(value string) int {
	count := 0
	for _, char := range value {
		if char != ' ' {
			break
		}
		count++
	}
	return count
}

func editorSubmissionNotice(args []string) string {
	if isVariableEditor(args) {
		return "Validating and saving plain and secret variables …"
	}
	if len(args) >= 2 && args[0] == "project" {
		switch args[1] {
		case "create":
			return "Validating YAML and creating the environment file …"
		case "apply":
			return "Validating and applying environment YAML …"
		case "validate":
			return "Validating environment YAML …"
		}
	}
	return "Validating YAML …"
}

func formatEditorYAML(args []string, content string) string {
	if len(args) != 4 || args[0] != "project" || args[1] != "create" {
		return content
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return content
	}
	normalizeYAMLStyle(&document)
	var formatted bytes.Buffer
	encoder := yaml.NewEncoder(&formatted)
	encoder.SetIndent(yamlIndentWidth)
	if err := encoder.Encode(&document); err != nil {
		return content
	}
	if err := encoder.Close(); err != nil {
		return content
	}
	return formatted.String()
}

func normalizeYAMLStyle(node *yaml.Node) {
	if node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode {
		node.Style = 0
	}
	for _, child := range node.Content {
		normalizeYAMLStyle(child)
	}
}

func wantsPaste(command grammar.Command, args []string) bool {
	if command.Input == grammar.InputVariables {
		return true
	}
	if command.Input != grammar.InputYAML {
		return false
	}
	if len(args) == 4 && args[0] == "project" && args[1] == "create" {
		return true
	}
	if len(args) == 0 || args[len(args)-1] != "-" {
		return false
	}
	return len(args) >= 2 && args[0] == "project" && (args[1] == "apply" || args[1] == "validate")
}

func (m *model) prepareEditor(args []string) {
	m.editorTitle = "YAML"
	m.editorHint = "Ctrl+S validate and apply · Esc cancel"
	m.editorError = ""
	m.editor.Placeholder = "Paste YAML here. Ctrl+S validates and applies; Esc cancels."
	m.editor.SetValue("")
	if isVariableEditor(args) {
		m.editorTitle = "VARIABLES · " + args[2]
		m.editorHint = "Ctrl+S validate and save · Esc cancel"
		m.editor.Placeholder = "NAME=value or secret NAME=value"
		return
	}
	if len(args) == 4 && args[0] == "project" && args[1] == "create" {
		m.editorTitle = "NEW PROJECT"
		m.editorHint = "Ctrl+S validate and create file · Esc cancel"
		m.editor.SetValue(projectTemplate(args[2]))
		m.editor.CursorStart()
	}
}

func isVariableEditor(args []string) bool {
	return len(args) == 3 && args[0] == "project" && args[1] == "variables"
}

func projectTemplate(project string) string {
	return fmt.Sprintf(`workflow: build_deploy
builder: builder-01
runners:
  - runner-01
services:
  %s:
    git: https://github.com/example/%s.git
    branch: main
    dockerfile: Dockerfile
    context: .
    ports:
      - "8080:8080"
    restart: unless-stopped
`, project, project)
}
