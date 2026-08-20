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

const (
	cardChrome  = 4
	cardMinimum = 12
)

func Card(title, body string, width int, accent bool) string {
	tone := theme.Rule
	if accent {
		tone = theme.Lead
	}

	inner := width - cardChrome
	if inner < cardMinimum {
		inner = cardMinimum
	}

	label := ""
	if title != "" {
		label = " " + theme.Heading.Render(theme.Upper(title)) + " "
	}

	fill := inner + 2 - lipgloss.Width(label)
	if fill < 0 {
		fill = 0
	}

	lines := []string{
		tone.Render(theme.BoxTopLeft) + label + tone.Render(strings.Repeat(theme.IconLine, fill)+theme.BoxTopRight),
	}

	edge := tone.Render(theme.BoxVertical)
	for _, line := range theme.Rows(body) {
		lines = append(lines, edge+" "+theme.Cell(line, inner)+" "+edge)
	}

	lines = append(lines,
		tone.Render(theme.BoxBottomLeft+strings.Repeat(theme.IconLine, inner+2)+theme.BoxBottomRight))

	return strings.Join(lines, "\n")
}

func CardWidth(width int) int {
	inner := width - cardChrome
	if inner < cardMinimum {
		return cardMinimum
	}
	return inner
}

func Divider(width int) string {
	if width <= 0 {
		return ""
	}
	return theme.Rule.Render(strings.Repeat(theme.IconLine, width))
}

func Stack(sections ...string) string {
	kept := make([]string, 0, len(sections))
	for _, section := range sections {
		if section != "" {
			kept = append(kept, section)
		}
	}
	return strings.Join(kept, "\n")
}
