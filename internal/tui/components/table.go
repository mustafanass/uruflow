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
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/urustack/uruflow/internal/tui/theme"
)

const (
	gutter      = "  "
	pointerCell = 2
)

type Column struct {
	Title string
	Width int
	Right bool
	Flex  bool
}

type Cell struct {
	Text  string
	Style lipgloss.Style
}

type Row []Cell

type Table struct {
	Columns []Column
	Rows    []Row
	Cursor  int
	Height  int
	Empty   string
}

func Text(value string) Cell {
	return Cell{Text: value, Style: theme.Body}
}

func Styled(value string, style lipgloss.Style) Cell {
	return Cell{Text: value, Style: style}
}

func (t Table) Render(width int) string {
	widths := t.layout(width)

	var out strings.Builder
	out.WriteString(t.header(widths))
	out.WriteString("\n")
	out.WriteString(Divider(width))
	out.WriteString("\n")

	if len(t.Rows) == 0 {
		empty := t.Empty
		if empty == "" {
			empty = "nothing here yet"
		}
		out.WriteString(strings.Repeat(" ", pointerCell) + theme.Ghost.Render(empty))
		return out.String()
	}

	start, end := window(len(t.Rows), t.Cursor, t.Height)
	for index := start; index < end; index++ {
		out.WriteString(t.row(index, widths))
		if index < end-1 {
			out.WriteString("\n")
		}
	}

	if end < len(t.Rows) {
		out.WriteString("\n" + strings.Repeat(" ", pointerCell) +
			theme.Ghost.Render(more(len(t.Rows)-end)))
	}

	return out.String()
}

func (t Table) header(widths []int) string {
	cells := make([]string, 0, len(t.Columns))
	for index, column := range t.Columns {
		cells = append(cells, theme.Column.Render(align(column, column.Title, widths[index])))
	}
	return strings.Repeat(" ", pointerCell) + strings.Join(cells, gutter)
}

func (t Table) row(index int, widths []int) string {
	row := t.Rows[index]
	selected := index == t.Cursor

	cells := make([]string, 0, len(t.Columns))
	for position, column := range t.Columns {
		value := ""
		style := theme.Body
		if position < len(row) {
			value = row[position].Text
			style = row[position].Style
		}
		if selected && position == 0 {
			style = theme.Selected
		}
		cells = append(cells, style.Render(align(column, value, widths[position])))
	}

	marker := strings.Repeat(" ", pointerCell)
	if selected {
		marker = theme.Lead.Render(theme.IconSelected) + " "
	}

	return marker + strings.Join(cells, gutter)
}

func (t Table) layout(width int) []int {
	widths := make([]int, len(t.Columns))

	used := pointerCell
	flex := -1
	for index, column := range t.Columns {
		widths[index] = column.Width
		if column.Flex {
			flex = index
		}
		used += column.Width
		if index > 0 {
			used += len(gutter)
		}
	}

	if flex >= 0 {
		widths[flex] += width - used
		if widths[flex] < 8 {
			widths[flex] = 8
		}
	}

	return widths
}

func align(column Column, value string, width int) string {
	if column.Right {
		return theme.PadLeft(theme.Truncate(value, width), width)
	}
	return theme.Cell(value, width)
}

func window(total, cursor, height int) (int, int) {
	if height <= 0 || height >= total {
		return 0, total
	}

	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}

func more(count int) string {
	if count == 1 {
		return "1 more row"
	}
	return strconv.Itoa(count) + " more rows"
}
