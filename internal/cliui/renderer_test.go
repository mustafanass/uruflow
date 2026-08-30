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
	"os"
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

func TestPreviewStatus(t *testing.T) {
	if os.Getenv("PREVIEW") == "" {
		t.Skip("set PREVIEW=1")
	}
	renderer := New(os.Stdout, os.Getenv("NO_COLOR") == "")
	renderer.Width = 110
	_ = renderer.Render(ops.Event{Type: ops.EventResult, Title: "fleet", Data: map[string]any{
		"agents_online": 2, "agents_total": 3, "projects": 5, "containers_running": 12,
		"releases_active": 1, "alerts": 0, "registry": "healthy",
	}})
	_ = renderer.Render(ops.Table("agents", []string{"NAME", "ROLES", "STATE", "CTR", "CPU", "MEMORY", "DISK", "SEEN"}, [][]string{
		{"builder-01", "builder,runner", "online", "4", "34%", "8.7 GB/14.9 GB", "81%", "now"},
		{"web-01", "runner", "online", "3", "12%", "2.1 GB/8 GB", "42%", "now"},
		{"web-02", "runner", "offline", "0", "–", "–", "–", "3m"},
	}))
	_ = renderer.Render(ops.Event{Type: ops.EventLog, Time: time.Date(2026, 8, 27, 14, 3, 10, 0, time.Local), Title: "web-01", Message: "container healthy in 8s"})
}
