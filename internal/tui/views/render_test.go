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

package views

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/urustack/uruflow/internal/models"
	"github.com/urustack/uruflow/internal/storage/sqlite"
	"github.com/urustack/uruflow/internal/tui/components"
)

const (
	renderWidth  = 150
	renderHeight = 40
)

func seed(t *testing.T) *sqlite.Store {
	t.Helper()

	store, err := sqlite.New(filepath.Join(t.TempDir(), "render.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	store.CreateAgent(&models.Agent{ID: "a1", Name: "builder-01", Key: "k",
		Roles: []models.Role{models.RoleBuilder}})
	store.CreateAgent(&models.Agent{ID: "a2", Name: "web-01", Key: "k",
		Roles: []models.Role{models.RoleRunner}})
	store.SetAgentStatus("a1", models.AgentOnline)
	store.SetAgentMetrics("a1", &models.Metrics{CPUPercent: 34, MemoryPercent: 62, DiskPercent: 91, Uptime: 96000})

	store.SaveProject(&models.Project{Name: "api", GitURL: "git@github.com:acme/api.git",
		Branch: "main", Builder: "a1", Runners: []string{"a2"}, AutoDeploy: true})

	started := time.Now().Add(-4 * time.Minute)
	ended := started.Add(96 * time.Second)
	store.CreateRelease(&models.Release{ID: "r1", Project: "api", Branch: "main",
		Commit: "0123456789abcdef", Image: "reg:5000/uruflow/api:0123456789ab",
		Status: models.StatusSucceeded, Builder: "a1", BuilderName: "builder-01",
		Trigger: models.TriggerWebhook, StartedAt: started})
	store.UpdateRelease(&models.Release{ID: "r1", Commit: "0123456789abcdef",
		Image: "reg:5000/uruflow/api:0123456789ab", Status: models.StatusSucceeded,
		EndedAt: &ended, Duration: 96000})

	store.CreateRelease(&models.Release{ID: "r2", Project: "api", Branch: "main",
		Status: models.StatusBuilding, Builder: "a1", BuilderName: "builder-01",
		Trigger: models.TriggerManual, StartedAt: time.Now()})

	store.ReplaceContainers("a2", []models.Container{{ID: "c1", Name: "uruflow-api",
		Project: "api", State: "running", StartedAt: started}})

	store.CreateAlert(&models.Alert{ID: "al1", AgentID: "a1", AgentName: "builder-01",
		Type: "high_disk", Message: "Disk usage above 85%",
		Severity: models.SeverityWarning, CreatedAt: time.Now().Add(-9 * time.Minute)})

	return store
}

func checkFits(t *testing.T, name, output string) {
	t.Helper()

	for index, line := range strings.Split(output, "\n") {
		if width := lipgloss.Width(line); width > renderWidth {
			t.Errorf("%s line %d overflows the viewport: %d > %d", name, index+1, width, renderWidth)
		}
	}
}

func TestOverviewRenders(t *testing.T) {
	page := NewOverview(seed(t))
	page.Resize(renderWidth, renderHeight)
	page.Init()

	output := page.Render()
	checkFits(t, "overview", output)

	for _, want := range []string{"FLEET", "RECENT RELEASES", "builder-01", "api"} {
		if !strings.Contains(output, want) {
			t.Errorf("overview is missing %q", want)
		}
	}
	t.Log("\n" + components.Header(renderWidth, []components.Tab{
		{Key: "1", Label: "overview"}, {Key: "2", Label: "projects"},
		{Key: "3", Label: "agents"}, {Key: "4", Label: "releases"},
		{Key: "5", Label: "registry"}, {Key: "6", Label: "alerts"},
	}, 0, "") + "\n" + output)
}

func TestAlertsRenders(t *testing.T) {
	page := NewAlerts(seed(t))
	page.Resize(renderWidth, renderHeight)
	page.Init()

	output := page.Render()
	checkFits(t, "alerts", output)

	if !strings.Contains(output, "Disk usage above 85%") {
		t.Error("alerts view does not show the alert message")
	}
	t.Log("\n" + output)
}

func TestSetupRenders(t *testing.T) {
	page := NewSetup(filepath.Join(t.TempDir(), "config.yaml"))
	page.Resize(renderWidth, renderHeight)

	output := page.Render()
	checkFits(t, "setup", output)

	if !strings.Contains(output, "FIRST RUN") {
		t.Error("setup wizard does not show its title")
	}
	t.Log("\n" + output)
}

func TestRuntimeSpecRoundTrip(t *testing.T) {
	ports, err := parsePorts("8080:80, 9000:9000/udp")
	if err != nil || len(ports) != 2 || ports[1].Protocol != "udp" {
		t.Fatalf("ports = %+v err = %v", ports, err)
	}
	if formatPorts(ports) != "8080:80,9000:9000/udp" {
		t.Fatalf("formatted ports = %q", formatPorts(ports))
	}

	volumes, err := parseVolumes("/srv/data:/data:ro,/etc/x:/etc/x")
	if err != nil || len(volumes) != 2 || !volumes[0].ReadOnly || volumes[1].ReadOnly {
		t.Fatalf("volumes = %+v err = %v", volumes, err)
	}

	if _, err := parsePorts("8080"); err == nil {
		t.Error("a port without a colon should be rejected")
	}
	if _, err := parseVolumes("/srv:/data:rw"); err == nil {
		t.Error("an unknown volume flag should be rejected")
	}
}
