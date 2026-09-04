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
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/mustafanass/uruflow/internal/cliui"
	"github.com/mustafanass/uruflow/internal/control"
	"github.com/mustafanass/uruflow/internal/ops"
)

type streamMsg struct {
	event *ops.Event
	err   error
	done  bool
}

type projectEditorMsg struct {
	args    []string
	content string
	err     error
}

type variableEditorMsg struct {
	args    []string
	content string
	err     error
}

const (
	maxTranscriptLines  = 5000
	keptTranscriptLines = 4000
)

type model struct {
	client            *control.Client
	viewport          viewport.Model
	input             textinput.Model
	editor            textarea.Model
	events            chan streamMsg
	cancel            context.CancelFunc
	history           []string
	historyPath       string
	transcript        string
	historyAt         int
	width             int
	height            int
	editorRows        int
	running           bool
	paste             bool
	confirm           bool
	pending           []string
	active            []string
	suggestionAt      int
	completionCache   map[string][]commandSpec
	completionLoading map[string]bool
	completionErrors  map[string]string
	editorTitle       string
	editorHint        string
	editorError       string
	editorSubmission  bool
	activityCursor    uint64
	color             bool
	initialized       bool
	pristine          bool
}

func Run(socket string, noColor bool) error {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return errors.New("the URUFLOW workspace requires an interactive terminal")
	}
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "Type a command or / to browse …"
	input.Focus()
	editor := textarea.New()
	editor.Placeholder = "Paste YAML here. Ctrl+S validates and saves; Esc cancels."
	editor.ShowLineNumbers = true
	editor.SetHeight(10)
	editor.SetWidth(80)
	color := !noColor && os.Getenv("NO_COLOR") == ""
	applyBrandStyles(&input, &editor, color)
	view := viewport.New(80, 20)
	historyPath := filepath.Join(filepath.Dir(socket), "console.history")
	history, _ := loadHistory(historyPath)
	m := &model{
		client: control.NewClient(socket), viewport: view, input: input, editor: editor,
		events: make(chan streamMsg, 64), color: color, pristine: true, history: history,
		historyPath: historyPath, historyAt: len(history),
		completionCache: make(map[string][]commandSpec), completionLoading: make(map[string]bool),
		completionErrors: make(map[string]string),
	}
	program := tea.NewProgram(m, tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func applyBrandStyles(input *textinput.Model, editor *textarea.Model, color bool) {
	if !color {
		return
	}
	accent := lipgloss.Color(cliui.BrandGold)
	ivory := lipgloss.Color(cliui.BrandIvory)
	muted := lipgloss.Color(cliui.BrandSteel)
	surface := lipgloss.Color(cliui.BrandNavy)

	input.PromptStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(muted)
	input.CompletionStyle = lipgloss.NewStyle().Foreground(muted)
	input.Cursor.Style = lipgloss.NewStyle().Foreground(accent)

	editor.FocusedStyle.CursorLine = lipgloss.NewStyle().Foreground(ivory).Background(surface)
	editor.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(accent).Background(surface).Bold(true)
	editor.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(muted)
	editor.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(muted)
	editor.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(accent)
	editor.BlurredStyle.LineNumber = lipgloss.NewStyle().Foreground(muted)
	editor.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(muted)
	editor.Cursor.Style = lipgloss.NewStyle().Foreground(accent)
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}
