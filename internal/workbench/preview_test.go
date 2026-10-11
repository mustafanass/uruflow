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

package workbench

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestPreviewWorkspace(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "Type a command or / to browse …"
	input.Focus()
	m := &model{
		input: input, editor: textarea.New(), viewport: viewport.New(88, 20),
		width: 90, height: 30, initialized: true,
	}
	m.resize()
	m.transcript = m.welcome()
	m.viewport.SetContent(m.transcript)
	fmt.Println(m.View())

	fmt.Println("\n--- command palette ---")
	m.input.SetValue("/")
	m.resize()
	fmt.Println(m.renderCommandArea())

	fmt.Println("\n--- deploy projects ---")
	m.input.SetValue("project deploy ")
	completion, _ := argumentCompletion(m.input.Value())
	m.completionCache = map[string][]commandSpec{completion.Key: {
		{Command: "project deploy api-prod", Summary: "build_deploy · prod", NeedsArgs: true},
		{Command: "project deploy website-prod", Summary: "deploy_only · prod", NeedsArgs: true},
	}}
	m.resize()
	fmt.Println(m.renderCommandArea())
}

func TestPreviewResponsiveCommandArea(t *testing.T) {
	for _, size := range []struct {
		width  int
		height int
	}{{24, 24}, {40, 24}, {80, 30}, {100, 30}, {120, 36}, {160, 40}} {
		input := textinput.New()
		input.Prompt = "› "
		input.SetValue("/")
		m := &model{
			input: input, editor: textarea.New(), viewport: viewport.New(max(20, size.width-2), 12),
			width: size.width, height: size.height, initialized: true,
		}
		m.resize()
		fmt.Printf("\n--- command palette %dx%d ---\n%s\n", size.width, size.height, m.renderCommandArea())
	}

	input := textinput.New()
	editor := textarea.New()
	m := &model{
		input: input, editor: editor, viewport: viewport.New(78, 18),
		width: 80, height: 30, initialized: true, paste: true,
		pending: []string{"project", "edit", "api-prod"},
	}
	m.prepareEditor(m.pending)
	m.editor.SetValue(strings.Join([]string{
		"workflow: build_deploy",
		"builder: builder-01",
		"runners:",
		"  - runner-01",
		"services:",
		"  api:",
		"    git: https://example.test/api.git",
	}, "\n"))
	m.editor.Focus()
	m.resize()
	fmt.Printf("\n--- project editor 80x30 ---\n%s\n", m.View())

	liveInput := textinput.New()
	liveInput.Prompt = "› "
	liveView := viewport.New(78, 18)
	live := &model{
		input: liveInput, editor: textarea.New(), viewport: liveView,
		width: 80, height: 24, initialized: true, running: true,
		active: []string{"events"}, cancel: func() {},
	}
	live.resize()
	history := make([]string, 30)
	for index := range history {
		history[index] = fmt.Sprintf("historical event %02d", index+1)
	}
	live.transcript = strings.Join(history, "\n")
	live.viewport.SetContent(live.transcript)
	live.viewport.GotoTop()
	live.append("\nrelease started\nbuild queued\n")
	fmt.Printf("\n--- live output while scrolled up 80x24 ---\n%s\n", live.View())
}
