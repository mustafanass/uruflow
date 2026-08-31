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
		{Command: "project deploy api-prod", Summary: "build_deploy · prod"},
		{Command: "project deploy website-prod", Summary: "deploy_only · prod"},
	}}
	m.resize()
	fmt.Println(m.renderCommandArea())
}
