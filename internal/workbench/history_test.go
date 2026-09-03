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

package workbench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandHistoryPersistsWithoutSensitiveValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.history")
	m := &model{historyPath: path}
	m.remember("status")
	m.remember("status")
	m.remember("agent add build-01 --key one-time-key")
	m.remember("project list")

	loaded, err := loadHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 || loaded[0] != "status" || loaded[1] != "project list" {
		t.Fatalf("history = %#v", loaded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("history permissions = %o", info.Mode().Perm())
	}
}

func TestHistoryIsBounded(t *testing.T) {
	m := &model{}
	for index := 0; index < maxHistoryEntries+20; index++ {
		m.remember("status " + string(rune('a'+index%20)))
	}
	if len(m.history) != maxHistoryEntries {
		t.Fatalf("history length = %d", len(m.history))
	}
}
