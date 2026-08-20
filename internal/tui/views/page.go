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

package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/urustack/uruflow/internal/tui/components"
)

type Page interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	Render() string
	Hints() []components.Hint
	Capturing() bool
	Resize(width, height int)
	Tick(frame int)
}

type Base struct {
	Width  int
	Height int
	Frame  int
	notice string
	tone   string
}

func (b *Base) Resize(width, height int) {
	b.Width = width
	b.Height = height
}

func (b *Base) Tick(frame int) { b.Frame = frame }

func (b *Base) Notify(text, tone string) {
	b.notice = text
	b.tone = tone
}

func (b *Base) Fail(err error) {
	if err == nil {
		return
	}
	b.Notify(err.Error(), "error")
}

func (b *Base) Clear() {
	b.notice = ""
	b.tone = ""
}

func (b *Base) Notice() string {
	if b.notice == "" {
		return ""
	}
	return components.Message(b.notice, b.tone)
}

func (b *Base) TableHeight(reserved int) int {
	height := b.Height - reserved
	if height < 3 {
		return 3
	}
	return height
}

func move(cursor, delta, total int) int {
	if total == 0 {
		return 0
	}

	cursor += delta
	if cursor < 0 {
		return total - 1
	}
	if cursor >= total {
		return 0
	}
	return cursor
}
