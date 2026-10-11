//go:build preview

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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/ops"
)

func TestPreviewStatus(t *testing.T) {
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

func TestPreviewResponsiveTables(t *testing.T) {
	for _, width := range []int{24, 40, 80, 120, 160} {
		renderer := New(os.Stdout, os.Getenv("NO_COLOR") == "")
		renderer.Width = width
		fmt.Printf("\n--- containers %d columns ---\n", width)
		_ = renderer.Render(ops.Table("containers", []string{
			"AGENT", "NAME", "TYPE", "PROJECT", "SERVICE", "ID", "STATE", "HEALTH", "CPU", "MEMORY",
		}, [][]string{
			{"web-01", "uruflow-api", "service", "api-prod", "api", "0123456789ab", "running", "healthy", "34%", "382 MiB"},
			{"web-02", "uruflow-worker-with-a-long-name", "service", "api-prod", "worker", "fedcba987654", "restarting", "starting", "2%", "74 MiB"},
		}))
	}
	renderer := New(os.Stdout, os.Getenv("NO_COLOR") == "")
	renderer.Width = 40
	fmt.Println("\n--- release detail 40 columns ---")
	_ = renderer.Render(ops.Table("release r1", []string{"FIELD", "VALUE"}, [][]string{
		{"project", "api-prod"}, {"state", "failed"}, {"message", "DATABASE_URL is required"},
	}))
	fmt.Println("\n--- wrapped diagnostic 40 columns ---")
	_ = renderer.Render(ops.Event{Type: ops.EventLog, Time: time.Date(2026, 8, 27, 14, 3, 10, 0, time.Local),
		Title: "runner-01", Level: "warning", Message: "uruflow-api │ DATABASE_URL is required before the service can start"})
}
