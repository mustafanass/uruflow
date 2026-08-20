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
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
	"github.com/mustafanass/uruflow/internal/tui/views"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	refreshInterval = 700 * time.Millisecond
	frameInterval   = 90 * time.Millisecond
	logQueue        = 256
)

type refreshMsg struct{}
type frameMsg struct{}

type Model struct {
	server *api.Server
	pages  []views.Page
	tabs   []components.Tab
	active int
	width  int
	height int
	ready  bool
	frame  int
	help   bool
	logs   chan views.ContainerLogMsg
}

func NewModel(server *api.Server) *Model {
	logs := make(chan views.ContainerLogMsg, logQueue)
	server.Link().Subscribe(&bridge{logs: logs})

	return &Model{
		server: server,
		logs:   logs,
		pages: []views.Page{
			views.NewOverview(server.Store()),
			views.NewProjects(server),
			views.NewAgents(server),
			views.NewReleases(server),
			views.NewRegistry(server),
			views.NewAlerts(server.Store()),
			views.NewSecrets(server),
		},
		tabs: []components.Tab{
			{Key: "1", Label: "overview"},
			{Key: "2", Label: "projects"},
			{Key: "3", Label: "agents"},
			{Key: "4", Label: "releases"},
			{Key: "5", Label: "registry"},
			{Key: "6", Label: "alerts"},
			{Key: "7", Label: "secrets"},
		},
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.pages[m.active].Init(), tick(), frame(), m.listen())
}

func (m *Model) page() views.Page { return m.pages[m.active] }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		m.ready = true
		for _, page := range m.pages {
			page.Resize(message.Width, m.bodyHeight())
		}
		return m, nil

	case frameMsg:
		m.frame++
		m.page().Tick(m.frame)
		return m, frame()

	case refreshMsg:
		return m, tea.Batch(m.page().Update(message), tick())

	case views.ContainerLogMsg:
		cmd := m.pages[2].Update(message)
		return m, tea.Batch(cmd, m.listen())

	case tea.KeyMsg:
		return m.key(message)
	}

	return m, m.page().Update(msg)
}

func (m *Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.page().Capturing() {
		return m, m.page().Update(msg)
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.help = !m.help
		return m, nil
	case "tab":
		return m, m.switchTo(m.active + 1)
	case "shift+tab":
		return m, m.switchTo(m.active - 1)
	case "1", "2", "3", "4", "5", "6", "7":
		return m, m.switchTo(int(msg.String()[0] - '1'))
	}

	if m.help {
		m.help = false
	}
	return m, m.page().Update(msg)
}

func (m *Model) switchTo(index int) tea.Cmd {
	m.help = false
	m.active = (index + len(m.pages)) % len(m.pages)
	m.page().Resize(m.width, m.bodyHeight())
	return m.page().Init()
}

func (m *Model) View() string {
	if !m.ready {
		return ""
	}

	header := components.Header(m.width, m.tabs, m.active, m.status())
	footer := components.Footer(m.width, m.page().Hints())

	body := m.page().Render()
	if m.help {
		body = m.renderHelp()
	}

	return components.Screen(m.width, m.height, header, body, footer)
}

func (m *Model) bodyHeight() int {
	height := m.height - 5
	if height < 6 {
		return 6
	}
	return height
}

func (m *Model) status() string {
	agents, err := m.server.Store().ListAgents()
	if err != nil {
		return ""
	}

	online := 0
	for _, agent := range agents {
		if agent.Status == models.AgentOnline {
			online++
		}
	}

	tone := theme.Good
	switch {
	case len(agents) == 0 || online == 0:
		tone = theme.Bad
	case online < len(agents):
		tone = theme.Warn
	}

	fleet := tone.Render(theme.IconOnline) +
		theme.Faint.Render(fmt.Sprintf(" %d/%d agents", online, len(agents)))

	registry := theme.Note.Render(theme.IconImage) +
		theme.Faint.Render(" "+m.server.Config().Registry.Address())

	return fleet + theme.Ghost.Render("   ") + registry
}

func (m *Model) renderHelp() string {
	rows := [][2]string{
		{"1 – 7", "jump straight to a view"},
		{"tab / shift+tab", "cycle through views"},
		{"↑ ↓ or k j", "move the selection"},
		{"enter", "open the selected item"},
		{"esc", "leave a form or drill-down"},
		{"?", "toggle this help"},
		{"q", "quit uruflow"},
	}

	lines := []string{"", theme.Title.Render("Keys"), ""}
	for _, row := range rows {
		lines = append(lines, "  "+theme.Key.Render(theme.Cell(row[0], 18))+theme.Faint.Render(row[1]))
	}

	lines = append(lines, "", theme.Title.Render("How a release flows"), "",
		"  "+theme.Faint.Render("webhook or manual deploy")+theme.Ghost.Render("  "+theme.IconArrow+"  ")+
			theme.Mark.Render("builder clones, builds, pushes")+theme.Ghost.Render("  "+theme.IconArrow+"  ")+
			theme.Note.Render("registry")+theme.Ghost.Render("  "+theme.IconArrow+"  ")+
			theme.Lead.Render("runners pull and run"),
		"",
		"  "+theme.Ghost.Render("runners never see your source; they only ever pull a tagged image"),
	)

	return strings.Join(lines, "\n")
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return refreshMsg{} })
}

func frame() tea.Cmd {
	return tea.Tick(frameInterval, func(time.Time) tea.Msg { return frameMsg{} })
}

func (m *Model) listen() tea.Cmd {
	return func() tea.Msg { return <-m.logs }
}

type bridge struct {
	logs chan views.ContainerLogMsg
}

func (b *bridge) AgentConnected(agent *models.Agent)             {}
func (b *bridge) AgentDisconnected(agentID string)               {}
func (b *bridge) JobLog(agentID string, entry ufp.JobLog)        {}
func (b *bridge) JobStatus(agentID string, status ufp.JobStatus) {}

func (b *bridge) ContainerLog(agentID string, entry ufp.ContainerLog) {
	select {
	case b.logs <- views.ContainerLogMsg(entry):
	default:
	}
}
