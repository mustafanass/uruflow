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
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/cliui"
	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/ops"
)

func (m *model) updateSecret(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+c":
		m.resetInputMode()
		m.append(m.paint(cliui.ANSIWarning, "▲ secret input cancelled") + "\n")
		return m, textinput.Blink
	case "enter":
		value := m.input.Value()
		args := append([]string{}, m.pending...)
		m.resetInputMode()
		if value == "" {
			m.append(m.paint(cliui.ANSIError, "✘ secret value cannot be empty") + "\n")
			return m, textinput.Blink
		}
		return m, m.start(args, value)
	}
	var command tea.Cmd
	m.input, command = m.input.Update(key)
	return m, command
}

func (m *model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+c":
		m.resetInputMode()
		m.append(m.paint(cliui.ANSIWarning, "▲ action cancelled") + "\n")
		return m, textinput.Blink
	case "enter":
		answer := strings.ToLower(strings.TrimSpace(m.input.Value()))
		args := append([]string{}, m.pending...)
		m.resetInputMode()
		if answer != "y" && answer != "yes" {
			m.append(m.paint(cliui.ANSIWarning, "▲ action cancelled") + "\n")
			return m, textinput.Blink
		}
		return m, m.start(args, "")
	}
	var command tea.Cmd
	m.input, command = m.input.Update(key)
	return m, command
}

func (m *model) resetInputMode() {
	m.secret, m.confirm, m.pending = false, false, nil
	m.input.SetValue("")
	m.input.Prompt = "› "
	m.input.Placeholder = "Type a command or / to browse …"
	m.input.EchoMode = textinput.EchoNormal
	m.input.Focus()
	m.resize()
}

func (m *model) start(args []string, input string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel, m.running = cancel, true
	m.active = append([]string{}, args...)
	m.input.Blur()
	go func() {
		err := m.client.Execute(ctx, args, input, func(event ops.Event) error {
			select {
			case m.events <- streamMsg{event: &event}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if errors.Is(err, context.Canceled) {
			err = nil
		}
		m.events <- streamMsg{err: err, done: true}
	}()
	return m.waitStream()
}

func (m *model) waitStream() tea.Cmd {
	return func() tea.Msg { return <-m.events }
}

func isDurableOperation(args []string) bool {
	command, err := grammar.Resolve(args)
	return err == nil && command.Durable
}

func split(line string) ([]string, error) {
	var args []string
	var word strings.Builder
	quote := rune(0)
	escaped := false
	flush := func() {
		if word.Len() > 0 {
			args = append(args, word.String())
			word.Reset()
		}
	}
	for _, char := range line {
		if escaped {
			word.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				word.WriteRune(char)
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == ' ' || char == '\t' {
			flush()
			continue
		}
		word.WriteRune(char)
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("unfinished quote or escape")
	}
	flush()
	return args, nil
}
