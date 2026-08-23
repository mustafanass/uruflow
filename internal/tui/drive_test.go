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

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/tui/views"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func driveServer(t *testing.T) *api.Server {
	t.Helper()

	if os.Getenv("URUFLOW_DOCKER_TESTS") == "" {
		t.Skip("set URUFLOW_DOCKER_TESTS=1 to drive the TUI")
	}

	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.UFPPort = 0
	cfg.Server.DataDir = dir
	cfg.Server.Advertise = "127.0.0.1"
	cfg.Registry.Host = "127.0.0.1"
	cfg.Registry.Port = 5601
	if err := cfg.Save(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	store, err := sqlite.New(filepath.Join(dir, "drive.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	store.CreateAgent(&models.Agent{ID: "b1", Name: "builder-01", Key: "k",
		Roles: []models.Role{models.RoleBuilder, models.RoleRunner}})
	store.SaveProject(&models.Project{Name: "api", GitURL: "git@host:api.git", Branch: "main",
		Builder: "b1", Runners: []string{"b1"}, AutoDeploy: true,
		Runtime: models.Runtime{Env: map[string]string{"MODE": "production"}}})

	server, err := api.NewServer(cfg, store)
	if err != nil {
		t.Skipf("server unavailable: %v", err)
	}
	return server
}

func press(t *testing.T, model *Model, keys ...string) string {
	t.Helper()

	for _, key := range keys {
		var msg tea.KeyMsg
		switch key {
		case "ctrl+t":
			msg = tea.KeyMsg{Type: tea.KeyCtrlT}
		case "ctrl+s":
			msg = tea.KeyMsg{Type: tea.KeyCtrlS}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		model.Update(msg)
	}
	return model.View()
}

func newDriven(t *testing.T) *Model {
	t.Helper()

	model := NewModel(driveServer(t))
	model.Update(tea.WindowSizeMsg{Width: 150, Height: 45})
	return model
}

func TestProjectsViewShowsDetailTabs(t *testing.T) {
	model := newDriven(t)

	view := press(t, model, "2")
	if !strings.Contains(view, "overview") || !strings.Contains(view, "variables") {
		t.Fatalf("detail tabs are not on screen:\n%s", view)
	}

	view = press(t, model, "ctrl+t")
	if !strings.Contains(view, "MODE") {
		t.Fatalf("ctrl+t did not reach the variables tab:\n%s", view)
	}
}

func TestLogBridgeCountsDroppedMessagesWithoutBlocking(t *testing.T) {
	logs := make(chan views.ContainerLogMsg, 1)
	bridge := &bridge{logs: logs}
	bridge.ContainerLog("a1", ufp.ContainerLog{Line: "first"})
	bridge.ContainerLog("a1", ufp.ContainerLog{Line: "second"})
	if dropped := bridge.dropped.Load(); dropped != 1 {
		t.Fatalf("dropped = %d", dropped)
	}
}

func TestCreateFlowShowsTabsAndPickers(t *testing.T) {
	model := newDriven(t)

	view := press(t, model, "2", "n")
	for _, want := range []string{"stored as", "standalone", "settings", "variables"} {
		if !strings.Contains(view, want) {
			t.Fatalf("create form is missing %q:\n%s", want, view)
		}
	}

	view = press(t, model, "ctrl+t")
	if !strings.Contains(view, "environment variables") {
		t.Fatalf("ctrl+t did not open the variables tab:\n%s", view)
	}

	view = press(t, model, "ctrl+t")
	if strings.Contains(view, "config") && strings.Contains(view, "written as the file") {
		t.Log("config tab reachable in file mode")
	}
}

func TestFileModeRevealsTheConfigTab(t *testing.T) {
	model := newDriven(t)

	press(t, model, "2", "n")
	view := press(t, model, "right")

	if !strings.Contains(view, "◉ file") {
		t.Fatalf("the stored-as picker did not switch to file:\n%s", view)
	}

	view = press(t, model, "ctrl+t", "ctrl+t")
	if !strings.Contains(view, "written as the file") {
		t.Fatalf("the config tab is not reachable in file mode:\n%s", view)
	}
}

func TestDetailTabsSurviveSmallTerminals(t *testing.T) {
	sizes := []struct{ width, height int }{
		{150, 45}, {120, 30}, {100, 24}, {80, 20},
	}

	for _, size := range sizes {
		model := NewModel(driveServer(t))
		model.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})

		view := press(t, model, "2")
		lines := strings.Split(view, "\n")

		hasTabs := strings.Contains(view, "overview") && strings.Contains(view, "variables")
		t.Logf("%dx%d -> rendered %d lines, detail tabs visible: %v",
			size.width, size.height, len(lines), hasTabs)

		if !hasTabs {
			t.Errorf("%dx%d: the detail tabs are cut off\n%s", size.width, size.height, view)
		}
	}
}

func TestDriveAgainstLiveSetup(t *testing.T) {
	if os.Getenv("URUFLOW_LIVE_DIR") == "" {
		t.Skip("set URUFLOW_LIVE_DIR to drive against a real install")
	}
	live := os.Getenv("URUFLOW_LIVE_DIR")

	cfg, err := config.Load(filepath.Join(live, "config.yaml"))
	if err != nil {
		t.Fatalf("load live config: %v", err)
	}

	store, err := sqlite.New(filepath.Join(live, "data", "uruflow.db"))
	if err != nil {
		t.Fatalf("open live db: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	server, err := api.NewServer(cfg, store)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	server.ReloadProjects()

	model := NewModel(server)
	model.Update(tea.WindowSizeMsg{Width: 150, Height: 40})

	t.Log("\n\n########## after pressing 2 (projects) ##########\n" + press(t, model, "2"))
	t.Log("\n\n########## after ctrl+t (variables) ##########\n" + press(t, model, "ctrl+t"))
	t.Log("\n\n########## after ctrl+t again (config) ##########\n" + press(t, model, "ctrl+t"))
}

func TestSecretsViewMasksValues(t *testing.T) {
	model := newDriven(t)

	sealed, _ := model.server.Vault().Seal("postgres://user:hunter2@db/api")
	model.server.Store().SetSecret("api_db", sealed)

	view := press(t, model, "7")

	if !strings.Contains(view, "api_db") {
		t.Fatalf("the secret name is not listed:\n%s", view)
	}
	if strings.Contains(view, "hunter2") {
		t.Fatalf("the secret VALUE leaked into the interface:\n%s", view)
	}
	if !strings.Contains(view, "${secret:api_db}") {
		t.Fatalf("the reference is not shown:\n%s", view)
	}
	if !strings.Contains(view, "••••") {
		t.Fatalf("the value is not masked:\n%s", view)
	}
}

func TestAddSecretEntirelyInTheInterface(t *testing.T) {
	model := newDriven(t)

	view := press(t, model, "7")
	if !strings.Contains(view, "no secrets stored") {
		t.Fatalf("expected an empty secrets view:\n%s", view)
	}

	view = press(t, model, "n")
	for _, want := range []string{"NEW SECRET", "name", "value"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the create form is missing %q:\n%s", want, view)
		}
	}

	press(t, model, "a", "p", "i", "_", "d", "b")
	press(t, model, "tab")
	press(t, model, "h", "u", "n", "t", "e", "r", "2")

	typing := model.View()
	if strings.Contains(typing, "hunter2") {
		t.Fatalf("the value is visible while typing:\n%s", typing)
	}
	if !strings.Contains(typing, "•") {
		t.Fatalf("the value field is not masked:\n%s", typing)
	}

	view = press(t, model, "enter")

	stored, err := model.server.Store().ListSecrets()
	if err != nil || len(stored) != 1 || stored[0].Name != "api_db" {
		t.Fatalf("secret was not stored: %+v err=%v", stored, err)
	}

	sealed, _ := model.server.Store().GetSecret("api_db")
	opened, err := model.server.Vault().Open(sealed)
	if err != nil || opened != "hunter2" {
		t.Fatalf("stored value = %q err = %v", opened, err)
	}

	if !strings.Contains(view, "${secret:api_db}") {
		t.Fatalf("the reference form is not shown after creating:\n%s", view)
	}
	if strings.Contains(view, "hunter2") {
		t.Fatalf("the value leaked into the list:\n%s", view)
	}
}

func TestRemoveSecretInTheInterface(t *testing.T) {
	model := newDriven(t)

	sealed, _ := model.server.Vault().Seal("value")
	model.server.Store().SetSecret("to_remove", sealed)

	press(t, model, "7")
	view := press(t, model, "d")
	if !strings.Contains(view, "Remove secret") {
		t.Fatalf("no confirmation was shown:\n%s", view)
	}

	press(t, model, "y")

	stored, _ := model.server.Store().ListSecrets()
	if len(stored) != 0 {
		t.Fatalf("the secret survived removal: %+v", stored)
	}
}
