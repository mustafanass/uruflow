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

const tabPadding = 2

type Tab struct {
	Key   string
	Label string
}

type TabItem struct {
	Label string
	Dirty bool
}

func TabBar(items []TabItem, active int, width int, note string) string {
	if len(items) == 0 {
		return Divider(width)
	}

	labels := make([]string, len(items))
	widths := make([]int, len(items))
	for index, item := range items {
		label := item.Label
		if item.Dirty {
			label += " " + theme.IconWarning
		}
		labels[index] = label
		widths[index] = lipgloss.Width(label) + tabPadding
	}

	strip := make([]string, 0, len(items))
	for index, label := range labels {
		padded := " " + label + " "
		if index == active {
			strip = append(strip, theme.TabActive.Padding(0, 0).Render(padded))
			continue
		}
		strip = append(strip, theme.TabIdle.Padding(0, 0).Render(padded))
	}

	bar := strings.Join(strip, "")
	if note != "" {
		gap := width - lipgloss.Width(bar) - lipgloss.Width(note)
		if gap < 1 {
			gap = 1
		}
		bar += strings.Repeat(" ", gap) + theme.Ghost.Render(theme.Truncate(note, width))
	}

	return bar + "\n" + tabRule(widths, active, width)
}

func tabRule(widths []int, active, width int) string {
	start := 0
	for index := 0; index < active && index < len(widths); index++ {
		start += widths[index]
	}

	span := 0
	if active >= 0 && active < len(widths) {
		span = widths[active]
	}

	rule := make([]rune, 0, width)
	for column := 0; column < width; column++ {
		switch {
		case column == start-1 && start > 0:
			rule = append(rule, []rune(theme.BoxTabLeft)[0])
		case column >= start && column < start+span:
			rule = append(rule, ' ')
		case column == start+span:
			rule = append(rule, []rune(theme.BoxTabRight)[0])
		default:
			rule = append(rule, []rune(theme.IconLine)[0])
		}
	}

	return theme.Rule.Render(string(rule))
}
