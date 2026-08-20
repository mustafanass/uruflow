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

package components

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

type Sheet struct {
	Label string
	Path  string
	area  textarea.Model
	saved string
}

func NewSheet(label, path string) *Sheet {
	area := textarea.New()
	area.Prompt = ""
	area.ShowLineNumbers = true
	area.CharLimit = 0
	area.MaxHeight = 0
	area.FocusedStyle.Base = theme.Base
	area.BlurredStyle.Base = theme.Base
	area.FocusedStyle.CursorLine = theme.Base
	area.FocusedStyle.LineNumber = theme.Ghost
	area.BlurredStyle.LineNumber = theme.Ghost
	area.FocusedStyle.Text = theme.Body
	area.BlurredStyle.Text = theme.Body
	area.Focus()

	return &Sheet{Label: label, Path: path, area: area}
}

func (s *Sheet) Load(content string) {
	s.area.SetValue(content)
	s.saved = content
}

func (s *Sheet) Value() string { return s.area.Value() }

func (s *Sheet) Dirty() bool { return s.area.Value() != s.saved }

func (s *Sheet) MarkSaved() { s.saved = s.area.Value() }

func (s *Sheet) Resize(width, height int) {
	s.area.SetWidth(width - 2)
	s.area.SetHeight(height)
}

func (s *Sheet) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.area, cmd = s.area.Update(msg)
	return cmd
}

func (s *Sheet) Render() string { return s.area.View() }
