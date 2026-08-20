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
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

const (
	setupAdvertise = iota
	setupRegistry
	setupUFPPort
	setupHTTPPort
	setupDataDir
)

type Setup struct {
	Base
	path string
	form *components.Form
	cfg  *config.Config
	done bool
}

func NewSetup(path string) *Setup {
	if path == "" {
		path = config.DefaultConfigPath
	}

	defaults := config.Default()
	return &Setup{
		path: path,
		form: components.NewForm(
			components.NewField("server host", "uruflow.internal", "hostname or IP that agents dial", false),
			components.NewField("registry host", "", "defaults to the server host", true),
			components.NewField("ufp port", strconv.Itoa(defaults.Server.UFPPort), "agent link port", true),
			components.NewField("webhook port", strconv.Itoa(defaults.Server.HTTPPort), "github and gitlab post here", true),
			components.NewField("data dir", defaults.Server.DataDir, "database, registry storage and PKI", true),
		),
	}
}

func (s *Setup) Init() tea.Cmd { return nil }

func (s *Setup) Capturing() bool { return !s.done }

func (s *Setup) Done() bool { return s.done }

func (s *Setup) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	if s.done {
		if key.String() == "enter" || key.String() == "q" {
			return tea.Quit
		}
		return nil
	}

	switch key.String() {
	case "tab", "down":
		s.form.Next()
		return nil
	case "shift+tab", "up":
		s.form.Previous()
		return nil
	case "enter":
		s.write()
		return nil
	case "ctrl+c", "esc":
		return tea.Quit
	}

	return s.form.Update(msg)
}

func (s *Setup) write() {
	if missing := s.form.Missing(); missing != "" {
		s.Notify(missing+" is required", "error")
		return
	}

	cfg := config.Default()
	cfg.Server.Advertise = s.form.Value(setupAdvertise)

	cfg.Registry.Host = s.form.Value(setupRegistry)
	if cfg.Registry.Host == "" {
		cfg.Registry.Host = cfg.Server.Advertise
	}

	if port, err := strconv.Atoi(s.form.Value(setupUFPPort)); err == nil && port > 0 {
		cfg.Server.UFPPort = port
	}
	if port, err := strconv.Atoi(s.form.Value(setupHTTPPort)); err == nil && port > 0 {
		cfg.Server.HTTPPort = port
	}
	if dataDir := s.form.Value(setupDataDir); dataDir != "" {
		cfg.Server.DataDir = dataDir
	}

	if err := cfg.Validate(); err != nil {
		s.Fail(err)
		return
	}
	if err := cfg.Save(s.path); err != nil {
		s.Fail(err)
		return
	}

	s.cfg = cfg
	s.done = true
}

func (s *Setup) Hints() []components.Hint {
	if s.done {
		return []components.Hint{{Key: "enter", Label: "start uruflow"}}
	}
	return []components.Hint{
		{Key: "tab", Label: "next field"},
		{Key: "enter", Label: "write config"},
		{Key: "esc", Label: "quit"},
	}
}

func (s *Setup) Render() string {
	if s.done {
		return s.renderDone()
	}

	body := strings.Join([]string{
		theme.Ghost.Render("uruflow builds images on a builder agent, pushes them to its own registry,"),
		theme.Ghost.Render("then releases those images to runner agents."),
		"",
		s.form.Render(components.CardWidth(s.Width)),
	}, "\n")

	return components.Stack(
		theme.Brand.Render(theme.Wordmark),
		"",
		components.Card("first run", body, s.Width, true),
		s.Notice(),
	)
}

func (s *Setup) renderDone() string {
	cfg := s.cfg

	return strings.Join([]string{
		"",
		theme.Good.Render(theme.IconSuccess + " configuration written to " + s.path),
		"",
		theme.Heading.Render("WHAT HAPPENS ON START"),
		"  " + components.KeyValue("pki", "a CA is generated in "+cfg.PKIDir(), 12),
		"  " + components.KeyValue("registry", cfg.Registry.Image+" starts on "+cfg.Registry.Address(), 12),
		"  " + components.KeyValue("link", "agents connect to "+fmt.Sprintf("%s:%d", cfg.Server.Advertise, cfg.Server.UFPPort), 12),
		"  " + components.KeyValue("webhooks", fmt.Sprintf("port %d%s", cfg.Server.HTTPPort, cfg.Webhook.Path), 12),
		"",
		theme.Heading.Render("NEXT"),
		theme.Ghost.Render("  1. enrol a builder and one or more runners from the agents view"),
		theme.Ghost.Render("  2. copy " + cfg.CACertPath() + " to each agent"),
		theme.Ghost.Render("  3. add a project and deploy it"),
		"",
		theme.Ghost.Render("  webhook secret  ") + theme.Note.Render(cfg.Webhook.Secret),
	}, "\n")
}
