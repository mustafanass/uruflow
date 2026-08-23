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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/tui/components"
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

func TestRawProjectYAMLRejectsUnknownServiceFields(t *testing.T) {
	content := "branch: main\nbuilder: builder-01\nrunners: [web-01]\nservices:\n  api:\n    healthcheck:\n      type: tcp\n      port: 80\n      intervaal: 2s\n"
	if err := validateYAML(content); err == nil || !strings.Contains(err.Error(), "intervaal") {
		t.Fatalf("error = %v", err)
	}
}

func TestServiceEditorAddsEditsAndRemovesAService(t *testing.T) {
	page := NewProjects(nil)
	page.Resize(100, 24)
	page.variables = components.NewSheet("variables", "")
	page.yaml = components.NewSheet("config", "")
	page.resetServiceEditor()
	page.openService(-1)
	page.serviceForm.Field(serviceFieldName).Set("api")
	page.runtimeForm.Field(runtimeFieldPorts).Set("8080:8080")
	page.healthForm.Field(healthFieldType).Set("http")
	page.healthForm.Field(healthFieldPath).Set("/ready")
	page.healthForm.Field(healthFieldPort).Set("8080")
	page.timingForm.Field(timingFieldInterval).Set("1s")
	page.timingForm.Field(timingFieldTimeout).Set("500ms")
	page.timingForm.Field(timingFieldRetries).Set("3")
	page.serviceLabels.Load("traefik.enable: \"true\"\n")
	page.saveService()

	if len(page.services) != 1 || page.services[0].Healthcheck == nil || page.services[0].Healthcheck.Path != "/ready" || page.services[0].Labels["traefik.enable"] != "true" {
		t.Fatalf("service = %+v", page.services)
	}
	page.openService(0)
	page.runtimeForm.Field(runtimeFieldCommand).Set("./api")
	page.saveService()
	if page.services[0].Command != "./api" {
		t.Fatalf("edited command = %q", page.services[0].Command)
	}
	page.serviceCursor = 0
	page.servicesKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if len(page.services) != 0 || !page.serviceDirty {
		t.Fatalf("remove failed: %+v", page.services)
	}
}

func TestMultiServiceDetailsShowRuntimeSummary(t *testing.T) {
	page := NewProjects(nil)
	page.Resize(80, 20)
	project := &models.Project{Services: []models.Service{
		{Name: "api", Dockerfile: "Dockerfile", Ports: []models.Port{{Host: 8080, Container: 80}}, Network: "edge", Healthcheck: &models.Healthcheck{Type: "http"}, Labels: map[string]string{"traefik.enable": "true"}},
		{Name: "cache", Image: "redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}}
	output := strings.Join(page.serviceDetails(project), "\n")
	for _, want := range []string{"api", "cache", "built", "image", "8080:80", "edge", "http", "labels 1"} {
		if !strings.Contains(output, want) {
			t.Fatalf("service details lack %q:\n%s", want, output)
		}
	}
	for index, line := range strings.Split(output, "\n") {
		if width := lipgloss.Width(line); width > components.CardWidth(80) {
			t.Fatalf("service detail line %d overflows: %d", index+1, width)
		}
	}
}

func TestServiceEditorRemainsCompactAtSmallWidths(t *testing.T) {
	page := NewProjects(nil)
	page.Resize(80, 20)
	page.variables = components.NewSheet("variables", "")
	page.yaml = components.NewSheet("config", "")
	page.resetServiceEditor()
	tabs := page.tabs()
	if len(tabs) < 3 || tabs[2].Label != "services" {
		t.Fatalf("services tab is unreachable: %+v", tabs)
	}
	page.openService(-1)
	output := page.renderServiceEditor()
	for index, line := range strings.Split(output, "\n") {
		if width := lipgloss.Width(line); width > components.CardWidth(80) {
			t.Fatalf("line %d overflows small editor: %d", index+1, width)
		}
	}
	if !strings.Contains(output, "settings") || !strings.Contains(output, "labels") {
		t.Fatalf("service tabs are not visible:\n%s", output)
	}
}

func TestStopRequiresConfirmation(t *testing.T) {
	page := NewProjects(nil)
	page.Resize(100, 24)
	page.projects = []models.Project{{Name: "api"}}
	page.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if page.mode != projectConfirmStop {
		t.Fatalf("mode = %v", page.mode)
	}
	if output := page.Render(); !strings.Contains(output, "Stop api on all runners?") {
		t.Fatalf("confirmation is missing:\n%s", output)
	}
	page.confirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, func() { t.Fatal("stop action ran on cancel") })
}

func TestStoreErrorsAreVisible(t *testing.T) {
	store := seed(t)
	page := NewOverview(store)
	page.Resize(renderWidth, renderHeight)
	page.Init()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	page.reload()
	if output := page.Render(); !strings.Contains(output, "load overview") {
		t.Fatalf("store error is hidden:\n%s", output)
	}
}

func TestDroppedLogIndicatorRenders(t *testing.T) {
	page := &Agents{mode: agentLogs, streaming: "container", dropped: 17}
	page.Resize(100, 20)
	if output := page.renderLogs(); !strings.Contains(output, "17 log lines dropped") {
		t.Fatalf("drop indicator is missing:\n%s", output)
	}
}
