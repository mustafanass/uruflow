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
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/grammar"
)

func TestApplyFromDashEntersPasteMode(t *testing.T) {
	command, _ := grammar.Resolve([]string{"project", "apply", "api", "prod", "-"})
	if !wantsPaste(command, []string{"project", "apply", "api", "prod", "-"}) {
		t.Fatal("project apply from stdin did not enter paste mode")
	}
	command, _ = grammar.Resolve([]string{"project", "apply", "api", "prod", "file.yaml"})
	if wantsPaste(command, []string{"project", "apply", "api", "prod", "file.yaml"}) {
		t.Fatal("file apply unexpectedly entered paste mode")
	}
}

func TestProjectCreateUsesTheInlineEditorTemplate(t *testing.T) {
	args := []string{"project", "create", "payments", "prod"}
	command, _ := grammar.Resolve(args)
	if !wantsPaste(command, args) {
		t.Fatal("project create did not enter the inline editor")
	}
	m := &model{editor: textarea.New()}
	m.prepareEditor(args)
	if m.editorTitle != "NEW PROJECT" || !strings.Contains(m.editor.Value(), "name: payments") ||
		!strings.Contains(m.editor.Value(), "services:") || !strings.Contains(m.editorHint, "create files") {
		t.Fatalf("editor title=%q hint=%q yaml=\n%s", m.editorTitle, m.editorHint, m.editor.Value())
	}
}

func TestEditorTextSurvivesServerValidationFailure(t *testing.T) {
	editor := textarea.New()
	editor.SetValue("project:\n  name: api\n")
	m := &model{
		editor: editor, editorSubmission: true, active: []string{"project", "create", "api", "prod"},
		completionCache: map[string][]commandSpec{}, completionLoading: map[string]bool{},
	}
	updated, _ := m.Update(streamMsg{err: fmt.Errorf("unknown agent runner-01"), done: true})
	got := updated.(*model)
	if !got.paste || strings.TrimSpace(got.editor.Value()) != "project:\n  name: api" || len(got.pending) != 4 || got.editorError != "unknown agent runner-01" {
		t.Fatalf("editor was not restored: paste=%v pending=%v error=%q yaml=%q", got.paste, got.pending, got.editorError, got.editor.Value())
	}
}

func TestYAMLEditorUsesSmartIndentation(t *testing.T) {
	m := &model{editor: textarea.New()}
	m.editor.SetValue("services:")
	m.insertYAMLNewline()
	m.editor.InsertString("app:")
	m.insertYAMLNewline()
	if got := m.editor.Value(); got != "services:\n  app:\n    " {
		t.Fatalf("mapping indentation = %q", got)
	}

	m.editor.SetValue("runners:\n  - runner-01")
	m.insertYAMLNewline()
	if got := m.editor.Value(); got != "runners:\n  - runner-01\n  - " {
		t.Fatalf("list indentation = %q", got)
	}
}

func TestYAMLEditorTabAndBackspaceUseSpaces(t *testing.T) {
	m := &model{editor: textarea.New()}
	m.editor.Focus()
	m.insertYAMLIndent()
	if m.editor.Value() != "  " {
		t.Fatalf("tab inserted %q", m.editor.Value())
	}
	m.editor.InsertString("  image: nginx")
	m.removeYAMLIndent()
	if m.editor.Value() != "  image: nginx" {
		t.Fatalf("shift+tab produced %q", m.editor.Value())
	}

	m.editor.SetValue("")
	pasted := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\timage: nginx"), Paste: true}
	_, _ = m.updatePaste(pasted)
	if m.editor.Value() != "  image: nginx" {
		t.Fatalf("pasted tab was not normalized: %q", m.editor.Value())
	}
}

func TestEditorSaveTransitionHasExplicitNotice(t *testing.T) {
	if got := editorSubmissionNotice([]string{"project", "create", "api", "prod"}); !strings.Contains(got, "creating project files") {
		t.Fatalf("create notice = %q", got)
	}
	if got := editorSubmissionNotice([]string{"project", "apply", "api", "prod", "-"}); !strings.Contains(got, "applying") {
		t.Fatalf("apply notice = %q", got)
	}
}

func TestProjectCreationYAMLIsFormattedOnSave(t *testing.T) {
	content := "project: {name: api, git: https://example.test/api.git}\nenvironment:\n workflow: build_only\n branch: main\n builder: builder-01\n services: {app: {dockerfile: Dockerfile}}\n"
	formatted := formatEditorYAML([]string{"project", "create", "api", "prod"}, content)
	if !strings.Contains(formatted, "project:\n  name: api") || !strings.Contains(formatted, "environment:\n  workflow: build_only") {
		t.Fatalf("project YAML was not normalized:\n%s", formatted)
	}
	if got := formatEditorYAML([]string{"project", "apply", "api", "prod", "-"}, content); got != content {
		t.Fatal("project apply YAML was unexpectedly rewritten")
	}
	broken := "project: ["
	if got := formatEditorYAML([]string{"project", "create", "api", "prod"}, broken); got != broken {
		t.Fatal("syntax-invalid YAML was rewritten")
	}
}
