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

func Panel(title, body string, width int, active bool) string {
	style := theme.Panel
	if active {
		style = theme.PanelActive
	}

	inner := width - 4
	if inner < 8 {
		inner = 8
	}

	content := body
	if title != "" {
		content = theme.Heading.Render(theme.Upper(title)) + "\n" + body
	}

	return style.Width(inner).Render(content)
}

type Metric struct {
	Label string
	Value string
	Tone  lipgloss.Style
}

func StatStrip(metrics []Metric, width int) string {
	if len(metrics) == 0 {
		return ""
	}

	cell := width / len(metrics)
	if cell < 10 {
		cell = 10
	}

	labels := make([]string, 0, len(metrics))
	values := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		labels = append(labels, theme.Ghost.Render(theme.Cell(theme.Upper(metric.Label), cell)))
		values = append(values, metric.Tone.Bold(true).Render(theme.Cell(metric.Value, cell)))
	}

	return strings.Join(labels, "") + "\n" + strings.Join(values, "")
}

func Gauge(percent float64, width int) string {
	if width < 4 {
		width = 4
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(percent / 100 * float64(width))
	tone := theme.Good
	switch {
	case percent >= 90:
		tone = theme.Bad
	case percent >= 75:
		tone = theme.Warn
	}

	return tone.Render(strings.Repeat(theme.IconBarFull, filled)) +
		theme.Ghost.Render(strings.Repeat(theme.IconBarEmpty, width-filled))
}

func KeyValue(label, value string, labelWidth int) string {
	return theme.Ghost.Render(theme.Cell(label, labelWidth)) + theme.Body.Render(value)
}
