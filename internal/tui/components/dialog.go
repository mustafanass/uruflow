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
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/urustack/uruflow/internal/tui/theme"
)

const dialogWidth = 54

type Dialog struct {
	Title   string
	Message string
	Detail  string
	Confirm string
	Cancel  string
	Danger  bool
}

func (d Dialog) Render(width, height int) string {
	tone := theme.Lead
	if d.Danger {
		tone = theme.Bad
	}

	body := []string{
		tone.Bold(true).Render(d.Title),
		"",
		theme.Body.Render(theme.Truncate(d.Message, dialogWidth)),
	}
	if d.Detail != "" {
		body = append(body, theme.Ghost.Render(theme.Truncate(d.Detail, dialogWidth)))
	}

	confirm := d.Confirm
	if confirm == "" {
		confirm = "confirm"
	}
	cancel := d.Cancel
	if cancel == "" {
		cancel = "cancel"
	}

	body = append(body, "",
		theme.Key.Render("y")+" "+theme.Hint.Render(confirm)+
			theme.Hint.Render("    ")+
			theme.Key.Render("n")+" "+theme.Hint.Render(cancel))

	box := theme.Panel.
		BorderForeground(tone.GetForeground()).
		Padding(1, 2).
		Width(dialogWidth).
		Render(strings.Join(body, "\n"))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
