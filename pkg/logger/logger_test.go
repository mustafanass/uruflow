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

package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLoggerAlsoWritesToStdout(t *testing.T) {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = original
		Init("", "info")
	})

	path := filepath.Join(t.TempDir(), "uruflow.log")
	if err := Init(path, "info"); err != nil {
		t.Fatalf("init: %v", err)
	}
	Info("agent connected")
	writer.Close()

	consoleOutput, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	fileOutput, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	for name, output := range map[string]string{
		"stdout": string(consoleOutput),
		"file":   string(fileOutput),
	} {
		if !strings.Contains(output, "agent connected") {
			t.Fatalf("%s output = %q", name, output)
		}
	}
}
