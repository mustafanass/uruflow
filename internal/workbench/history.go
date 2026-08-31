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
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/mustafanass/uruflow/internal/cliui"
)

const maxHistoryEntries = 200

func loadHistory(path string) ([]string, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := make([]string, 0, maxHistoryEntries)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && safeForHistory(line) {
			entries = append(entries, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(entries) > maxHistoryEntries {
		entries = entries[len(entries)-maxHistoryEntries:]
	}
	return entries, nil
}

func saveHistory(path string, entries []string) error {
	if len(entries) > maxHistoryEntries {
		entries = entries[len(entries)-maxHistoryEntries:]
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".console-history-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	for _, entry := range entries {
		if _, err := temporary.WriteString(entry + "\n"); err != nil {
			temporary.Close()
			return err
		}
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func safeForHistory(line string) bool {
	args, err := split(line)
	if err != nil || len(args) == 0 {
		return false
	}
	if len(args) >= 2 && args[0] == "secret" && args[1] == "set" {
		return false
	}
	for _, argument := range args {
		name := strings.ToLower(strings.SplitN(argument, "=", 2)[0])
		switch name {
		case "--key", "--password", "--secret", "--token":
			return false
		}
	}
	return true
}

func (m *model) previousHistory() {
	if len(m.history) == 0 {
		return
	}
	m.historyAt = max(0, m.historyAt-1)
	m.input.SetValue(m.history[m.historyAt])
	m.input.CursorEnd()
}

func (m *model) remember(line string) {
	if !safeForHistory(line) {
		m.historyAt = len(m.history)
		return
	}
	if len(m.history) == 0 || m.history[len(m.history)-1] != line {
		m.history = append(m.history, line)
	}
	if len(m.history) > maxHistoryEntries {
		m.history = m.history[len(m.history)-maxHistoryEntries:]
	}
	m.historyAt = len(m.history)
	if m.historyPath != "" {
		if err := saveHistory(m.historyPath, m.history); err != nil {
			m.append(m.paint(cliui.ANSIWarning, "▲ command history was not saved: "+cliui.SafeText(err.Error())) + "\n")
		}
	}
}

func (m *model) nextHistory() {
	if len(m.history) == 0 {
		return
	}
	m.historyAt = min(len(m.history), m.historyAt+1)
	if m.historyAt == len(m.history) {
		m.input.SetValue("")
		return
	}
	m.input.SetValue(m.history[m.historyAt])
	m.input.CursorEnd()
}
