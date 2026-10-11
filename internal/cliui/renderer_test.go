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

package cliui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/mustafanass/uruflow/internal/ops"
)

func TestTableNeverExceedsTerminalWidth(t *testing.T) {
	for _, width := range []int{24, 32, 40, 60, 80, 100, 140} {
		buffer := &bytes.Buffer{}
		renderer := New(buffer, false)
		renderer.Width = width
		event := ops.Table("agents", []string{"NAME", "ROLES", "STATE", "CONTAINERS", "MEMORY", "DETAIL"}, [][]string{
			{"builder-01", "builder,runner", "online", "12", "8.7 GB/14.9 GB", strings.Repeat("long detail ", 12)},
		})
		if err := renderer.Render(event); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(buffer.String(), "\n"), "\n")
		for index, line := range lines {
			if got := ansi.StringWidth(line); got > width {
				t.Fatalf("width %d row %d is %d columns", width, index+1, got)
			}
		}
		if got := ansi.StringWidth(lines[0]); got != width {
			t.Fatalf("width %d top border is %d columns", width, got)
		}
		if got := ansi.StringWidth(lines[len(lines)-1]); got != width {
			t.Fatalf("width %d bottom border is %d columns", width, got)
		}
	}
}

func TestEmptyTableUsesFullWidthAndCentersMessage(t *testing.T) {
	renderer := New(nil, false)
	renderer.Width = 80
	output := renderer.RenderString(ops.Table("containers", []string{
		"AGENT", "PROJECT", "SERVICE", "ID", "STATE", "HEALTH", "CPU", "MEMORY",
	}, nil))
	lines := strings.Split(output, "\n")
	for index, line := range lines {
		if got := ansi.StringWidth(line); got != renderer.Width {
			t.Fatalf("row %d is %d columns, want %d", index+1, got, renderer.Width)
		}
	}
	emptyRow := ansi.Strip(lines[2])
	left := strings.Index(emptyRow, "no records")
	right := ansi.StringWidth(emptyRow) - ansi.StringWidth(emptyRow[:left]) - len("no records")
	left = ansi.StringWidth(emptyRow[:left])
	if difference := left - right; difference < -1 || difference > 1 {
		t.Fatalf("empty message is not centered: left=%d right=%d", left, right)
	}
}

func TestNarrowTableKeepsOperationalColumns(t *testing.T) {
	renderer := New(nil, false)
	renderer.Width = 32
	output := renderer.RenderString(ops.Table("containers", []string{
		"AGENT", "NAME", "TYPE", "PROJECT", "SERVICE", "ID", "STATE", "HEALTH", "CPU", "MEMORY",
	}, [][]string{{
		"web-01", "uruflow-api", "service", "api-prod", "api", "0123456789ab", "exited", "unhealthy", "0%", "0 B",
	}}))
	plain := ansi.Strip(output)
	for _, expected := range []string{"AGENT", "NAME", "STATE", "HEALTH", "exited", "unheal", "+6"} {
		if !strings.Contains(plain, expected) {
			t.Fatalf("narrow table lost %q:\n%s", expected, plain)
		}
	}
	for index, line := range strings.Split(output, "\n") {
		if got := ansi.StringWidth(line); got > renderer.Width {
			t.Fatalf("row %d is %d columns, want at most %d", index+1, got, renderer.Width)
		}
	}
}

func TestDetailTableUsesQuietLabelValueHierarchy(t *testing.T) {
	renderer := New(nil, true)
	renderer.Width = 48
	output := renderer.RenderString(ops.Table("release r1", []string{"FIELD", "VALUE"}, [][]string{
		{"project", "api-prod"}, {"state", "failed"}, {"message", "DATABASE_URL is required"},
	}))
	plain := ansi.Strip(output)
	if strings.Contains(plain, "FIELD") || strings.Contains(plain, "VALUE") {
		t.Fatalf("detail table kept redundant headings:\n%s", plain)
	}
	for _, expected := range []string{"project", "api-prod", "state", "failed", "DATABASE_URL is required"} {
		if !strings.Contains(plain, expected) {
			t.Fatalf("detail table lost %q:\n%s", expected, plain)
		}
	}
	if !strings.Contains(output, "\x1b["+ANSIMuted+"m") || !strings.Contains(output, "\x1b["+ANSIError+"m") {
		t.Fatalf("detail hierarchy is not colored semantically:\n%s", output)
	}
}

func TestLogsAndMessagesWrapWithinTerminalWidth(t *testing.T) {
	for _, width := range []int{24, 40, 80} {
		renderer := New(nil, true)
		renderer.Width = width
		events := []ops.Event{
			{Type: ops.EventLog, Time: time.Now(), Title: "runner-界", Level: "warning", Message: strings.Repeat("missing DATABASE_URL ", 8)},
			{Type: ops.EventMessage, Time: time.Now(), Operation: "release-01", Level: "error", Message: strings.Repeat("container failed readiness ", 6)},
		}
		for _, event := range events {
			output := renderer.RenderString(event)
			for index, line := range strings.Split(output, "\n") {
				if got := ansi.StringWidth(line); got > width {
					t.Fatalf("width %d row %d is %d columns:\n%s", width, index+1, got, output)
				}
			}
		}
	}
}

func TestTableStatesUseOperationalColors(t *testing.T) {
	renderer := New(nil, true)
	renderer.Width = 72
	output := renderer.RenderString(ops.Table("containers", []string{"NAME", "STATE", "HEALTH"}, [][]string{
		{"api", "running", "healthy"},
		{"worker", "restarting", "starting"},
		{"jobs", "exited", "unhealthy"},
	}))
	for _, code := range []string{ANSISuccess, ANSIWarning, ANSIError} {
		if !strings.Contains(output, "\x1b["+code+"m") {
			t.Fatalf("table does not contain semantic color %q:\n%s", code, output)
		}
	}
}

func TestAgentEnrollmentRendersCopyableCommand(t *testing.T) {
	renderer := New(nil, false)
	renderer.Width = 90
	output := renderer.RenderString(ops.Event{Type: ops.EventResult, Title: "agent enrolled", Data: map[string]any{
		"name": "build-01", "roles": "builder,runner", "id": "agent-id", "key": "one-time-key",
		"server": "uruflow.internal:9001", "ca_certificate": "/var/lib/uruflow/pki/ca.crt",
	}})
	for _, expected := range []string{
		"build-01  BUILDER,RUNNER",
		"--id agent-id",
		"--key one-time-key",
		"--server uruflow.internal:9001",
		"--roles builder,runner",
		"only available in this enrollment response",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("enrollment output does not contain %q:\n%s", expected, output)
		}
	}
	for index, line := range strings.Split(output, "\n") {
		if got := ansi.StringWidth(line); got != renderer.Width {
			t.Fatalf("row %d is %d columns, want %d", index+1, got, renderer.Width)
		}
	}
}

func TestBrandPaletteMatchesPublishedIdentity(t *testing.T) {
	want := map[string]string{
		"navy": BrandNavy, "blue": BrandBlue, "gold": BrandGold,
		"ivory": BrandIvory, "steel": BrandSteel,
	}
	expected := map[string]string{
		"navy": "#0B2444", "blue": "#1D3E6B", "gold": "#EDC35D",
		"ivory": "#F5F1E9", "steel": "#9FB3D1",
	}
	for name, value := range want {
		if value != expected[name] {
			t.Fatalf("%s = %s, want %s", name, value, expected[name])
		}
	}
	if got := Wordmark(false); got != PlainWordmark {
		t.Fatalf("plain wordmark = %q, want %q", got, PlainWordmark)
	}
	if PlainWordmark != "uruflow" {
		t.Fatalf("terminal wordmark = %q", PlainWordmark)
	}
	colored := Wordmark(true)
	if !strings.Contains(colored, "\x1b["+ANSIAccentBold+"m") || !strings.Contains(colored, "\x1b["+ANSITextBold+"m") {
		t.Fatalf("colored wordmark does not use the brand palette: %q", colored)
	}
}

func TestColoredPanelsUseBrandAccentAndBorder(t *testing.T) {
	renderer := New(nil, true)
	renderer.Width = 48
	output := renderer.RenderString(ops.Table("agents", []string{"NAME", "STATE"}, [][]string{{"runner-01", "online"}}))
	for _, code := range []string{ANSIAccentBold, ANSIBorder, ANSISuccess} {
		if !strings.Contains(output, "\x1b["+code+"m") {
			t.Fatalf("panel does not contain palette code %q:\n%s", code, output)
		}
	}
}

func TestRendererStripsTerminalInstructionsFromUntrustedOutput(t *testing.T) {
	renderer := New(nil, true)
	renderer.Width = 72
	attack := "safe\x1b[2J\x1b]52;c;stolen\a spoof\rhidden\u202e"
	for _, event := range []ops.Event{
		{Type: ops.EventLog, Title: attack, Message: attack},
		ops.Table(attack, []string{attack}, [][]string{{attack}}),
		{Type: ops.EventResult, Title: "result", Data: map[string]any{"detail": attack, "nested": map[string]any{"value": attack}}},
	} {
		output := renderer.RenderString(event)
		for _, forbidden := range []string{"\x1b[2J", "]52;", "\a", "\r", "\u202e"} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("rendered terminal control %q in %q", forbidden, output)
			}
		}
		if !strings.Contains(output, "safe") || !strings.Contains(output, "spoof") {
			t.Fatalf("printable output was lost: %q", output)
		}
	}
	colored := renderer.RenderString(ops.Event{Type: ops.EventMessage, Level: "success", Message: attack})
	if !strings.Contains(colored, "\x1b["+ANSISuccess+"m") {
		t.Fatalf("renderer-owned color was removed: %q", colored)
	}
}
