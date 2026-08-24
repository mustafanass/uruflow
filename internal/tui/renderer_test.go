/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 */

package tui

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

func TestAttachedRendererUsesTerminalEnvironmentAndRestores(t *testing.T) {
	previousRenderer := lipgloss.DefaultRenderer()
	previousBrand := theme.Brand.Render("URUFLOW")

	restore := bindTerminalRenderer(io.Discard, []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	})
	defer restore()
	if lipgloss.DefaultRenderer() == previousRenderer {
		t.Fatal("attached renderer did not become the default")
	}
	if rendered := theme.Brand.Render("URUFLOW"); !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("attached renderer omitted terminal styling: %q", rendered)
	}

	restore()
	if lipgloss.DefaultRenderer() != previousRenderer {
		t.Fatal("previous default renderer was not restored")
	}
	if rendered := theme.Brand.Render("URUFLOW"); rendered != previousBrand {
		t.Fatalf("shared theme renderer was not restored: got %q, want %q", rendered, previousBrand)
	}
}

func TestAttachedEnvironmentUsesLastValue(t *testing.T) {
	environment := attachedEnvironment{"TERM=xterm", "TERM=xterm-256color"}
	if term := environment.Getenv("TERM"); term != "xterm-256color" {
		t.Fatalf("TERM = %q", term)
	}
	if missing := environment.Getenv("MISSING"); missing != "" {
		t.Fatalf("MISSING = %q", missing)
	}
}
