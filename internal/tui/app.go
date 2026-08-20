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

package tui

import (
	"fmt"
	"io"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urustack/uruflow/internal/api"
	"github.com/urustack/uruflow/internal/tui/components"
	"github.com/urustack/uruflow/internal/tui/views"
)

func Run(server *api.Server) error {
	log.SetOutput(io.Discard)

	program := tea.NewProgram(NewModel(server), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

func RunSetup(path string) error {
	log.SetOutput(io.Discard)

	program := tea.NewProgram(&setupModel{page: views.NewSetup(path)}, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

type setupModel struct {
	page   *views.Setup
	width  int
	height int
	ready  bool
}

func (s *setupModel) Init() tea.Cmd { return s.page.Init() }

func (s *setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		s.width = size.Width
		s.height = size.Height
		s.ready = true
		s.page.Resize(size.Width, size.Height-4)
		return s, nil
	}
	return s, s.page.Update(msg)
}

func (s *setupModel) View() string {
	if !s.ready {
		return ""
	}

	header := components.Header(s.width, nil, -1, "")
	footer := components.Footer(s.width, s.page.Hints())
	return components.Screen(s.width, s.height, header, s.page.Render(), footer)
}
