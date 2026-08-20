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
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/urustack/uruflow/internal/tui/theme"
)

const cardTestWidth = 80

const (
	red   = "\x1b[31m"
	green = "\x1b[32m"
	reset = "\x1b[0m"
)

func paint(color, text string) string {
	return color + text + reset
}

func brokenEscape(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] != 0x1b {
			continue
		}

		cursor := index + 1
		if cursor >= len(text) || text[cursor] != '[' {
			return true
		}

		cursor++
		for cursor < len(text) && text[cursor] >= 0x30 && text[cursor] <= 0x3f {
			cursor++
		}
		for cursor < len(text) && text[cursor] >= 0x20 && text[cursor] <= 0x2f {
			cursor++
		}
		if cursor >= len(text) || text[cursor] < 0x40 || text[cursor] > 0x7e {
			return true
		}
		index = cursor
	}
	return false
}

func TestCardNeverOverflowsWithStyledContent(t *testing.T) {
	long := paint(green, "22:42:32") + " " +
		paint(red, strings.Repeat("nginx error open() failed for a very long path ", 6))

	card := Card("logs", long, cardTestWidth, false)

	for index, line := range strings.Split(card, "\n") {
		if width := ansi.StringWidth(line); width != cardTestWidth {
			t.Errorf("line %d has display width %d, want exactly %d", index+1, width, cardTestWidth)
		}
		if brokenEscape(line) {
			t.Errorf("line %d holds a truncated escape sequence: %q", index+1, line)
		}
	}
}

func TestTruncateKeepsEscapeSequencesIntact(t *testing.T) {
	styled := paint(red, "error") + paint(green, " ok") + paint(red, " note")

	for width := 1; width <= ansi.StringWidth(styled)+2; width++ {
		out := theme.Truncate(styled, width)

		if got := ansi.StringWidth(out); got > width {
			t.Fatalf("width %d: truncated display width is %d", width, got)
		}
		if brokenEscape(out) {
			t.Fatalf("width %d: output holds a truncated escape sequence %q", width, out)
		}
	}
}

func TestCardTitleKeepsStyledContentIntact(t *testing.T) {
	spinner := "\x1b[38;5;42m" + "\u28cb" + reset
	title := "uruflow-demo  " + spinner + paint(green, " streaming")

	card := Card(title, "one log line", cardTestWidth, true)

	if strings.Contains(card, "42M") || strings.Contains(card, "[0M") {
		t.Fatalf("a title escape terminator was uppercased into a CSI Delete-Line: %q", card)
	}
	if !strings.Contains(card, "\x1b[38;5;42m") {
		t.Fatalf("the styled spinner escape was mangled: %q", card)
	}
	if !strings.Contains(card, "URUFLOW-DEMO") {
		t.Fatalf("plain title text was not uppercased: %q", card)
	}
	for index, line := range strings.Split(card, "\n") {
		if brokenEscape(line) {
			t.Errorf("line %d holds a truncated escape sequence: %q", index+1, line)
		}
	}
}

func TestUpperLeavesEscapesAlone(t *testing.T) {
	styled := paint(red, "error") + " plain " + paint(green, "ok")

	upper := theme.Upper(styled)

	if strings.Contains(upper, "[31M") || strings.Contains(upper, "[0M") || strings.Contains(upper, "[32M") {
		t.Fatalf("Upper corrupted an escape terminator: %q", upper)
	}
	if !strings.Contains(upper, "ERROR") || !strings.Contains(upper, "PLAIN") || !strings.Contains(upper, "OK") {
		t.Fatalf("Upper failed to uppercase visible text: %q", upper)
	}
	if brokenEscape(upper) {
		t.Fatalf("Upper produced a broken escape: %q", upper)
	}
}

func TestSanitizeStripsControlSequences(t *testing.T) {
	hostile := "before\x1b[2J\x1b[H\x1b[Aafter\r\ntail\x07"

	clean := theme.Sanitize(hostile)

	for _, forbidden := range []string{"\x1b", "\r", "\n", "\x07"} {
		if strings.Contains(clean, forbidden) {
			t.Errorf("sanitised output still contains %q: %q", forbidden, clean)
		}
	}
	if !strings.Contains(clean, "before") || !strings.Contains(clean, "after") {
		t.Errorf("sanitising dropped visible text: %q", clean)
	}
}

func TestTableRowsFitTheirWidth(t *testing.T) {
	table := Table{
		Columns: []Column{
			{Title: "stage", Width: 8},
			{Title: "line", Width: 20, Flex: true},
		},
		Rows: []Row{
			{Text(paint(green, "build")), Text(paint(red, strings.Repeat("a very long log line ", 20)))},
		},
		Cursor: 0,
	}

	for index, line := range strings.Split(table.Render(cardTestWidth), "\n") {
		if width := ansi.StringWidth(line); width > cardTestWidth {
			t.Errorf("row %d overflows: %d > %d", index+1, width, cardTestWidth)
		}
		if brokenEscape(line) {
			t.Errorf("row %d holds a truncated escape sequence: %q", index+1, line)
		}
	}
}
