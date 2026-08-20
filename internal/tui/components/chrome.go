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
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

const (
	BrandMark = "◆ URUFLOW"
	minWidth  = 40
)

type Hint struct {
	Key   string
	Label string
}

func Header(width int, tabs []Tab, active int, status string) string {
	brand := theme.Brand.Render(BrandMark)

	rendered := make([]string, 0, len(tabs))
	for index, tab := range tabs {
		style := theme.TabIdle
		if index == active {
			style = theme.TabActive
		}
		rendered = append(rendered, style.Render(tab.Key+" "+tab.Label))
	}
	nav := strings.Join(rendered, "")

	left := brand + "  " + nav
	gap := width - lipgloss.Width(left) - lipgloss.Width(status)
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + status + "\n" + theme.Line(width)
}

func Footer(width int, hints []Hint) string {
	rendered := make([]string, 0, len(hints))
	for _, hint := range hints {
		rendered = append(rendered, theme.Key.Render(hint.Key)+" "+theme.Hint.Render(hint.Label))
	}

	return theme.Line(width) + "\n" + theme.Truncate(strings.Join(rendered, theme.Hint.Render("   ")), width)
}

func Screen(width, height int, header, body, footer string) string {
	if width < minWidth {
		return theme.Faint.Render("terminal too narrow")
	}

	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	bodyHeight := height - headerHeight - footerHeight - 2

	lines := theme.Rows(body)
	if bodyHeight > 0 && len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}

	return header + "\n\n" + strings.Join(lines, "\n") + "\n" + footer
}

func Message(text, tone string) string {
	switch tone {
	case "error":
		return theme.Bad.Render(theme.IconFailure + " " + text)
	case "warn":
		return theme.Warn.Render(theme.IconWarning + " " + text)
	case "success":
		return theme.Good.Render(theme.IconSuccess + " " + text)
	default:
		return theme.Faint.Render(theme.IconSeparator + " " + text)
	}
}
