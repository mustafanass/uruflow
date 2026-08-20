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

	tea "github.com/charmbracelet/bubbletea"
)

func TestSheetTracksDirtyAgainstSavedContent(t *testing.T) {
	sheet := NewSheet("config", "/tmp/dev.yaml")
	sheet.Load("branch: main\n")

	if sheet.Dirty() {
		t.Fatal("a freshly loaded sheet is dirty")
	}

	sheet.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if !sheet.Dirty() {
		t.Fatal("typing did not mark the sheet dirty")
	}

	sheet.MarkSaved()
	if sheet.Dirty() {
		t.Fatal("the sheet is still dirty after being marked saved")
	}
}

func TestEditorPasteLandsInTheActiveSheet(t *testing.T) {
	config := NewSheet("config", "/tmp/dev.yaml")
	environment := NewSheet(".env", "/tmp/dev.env")
	config.Load("")
	environment.Load("")

	config.Resize(60, 10)
	environment.Resize(60, 10)

	pasted := "LOG_LEVEL=debug"
	for _, symbol := range pasted {
		environment.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{symbol}})
	}

	if !strings.Contains(environment.Value(), pasted) {
		t.Fatalf(".env sheet holds %q", environment.Value())
	}
	if config.Value() != "" {
		t.Fatalf("input leaked into the inactive sheet: %q", config.Value())
	}
}

func TestTabBarMarksDirtySheets(t *testing.T) {
	clean := TabBar([]TabItem{{Label: "settings"}, {Label: ".env"}}, 0, 60, "")
	dirty := TabBar([]TabItem{{Label: "settings"}, {Label: ".env", Dirty: true}}, 1, 60, "note")

	if strings.Contains(clean, "▲") {
		t.Error("a clean tab bar shows the dirty marker")
	}
	if !strings.Contains(dirty, "▲") {
		t.Error("a dirty tab is not marked")
	}
	if !strings.Contains(dirty, "note") {
		t.Error("the tab bar note is missing")
	}
}
