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
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/helper"
)

type agentMode int

const (
	agentList agentMode = iota
	agentCreate
	agentEnrolled
	agentConfirmDelete
	agentContainers
	agentLogs
)

const (
	logBuffer     = 400
	requestWindow = 10 * time.Second
	logTail       = 200
)

type ContainerLogMsg ufp.ContainerLog

type Agents struct {
	Base
	server *api.Server
	mode   agentMode
	cursor int
	agents []models.Agent

	containerCursor int
	containers      []models.Container
	streaming       string
	lines           []string

	form     *components.Form
	enrolled *models.Agent
}

func NewAgents(server *api.Server) *Agents {
	return &Agents{
		server: server,
		form: components.NewForm(
			components.NewField("name", "builder-01", "unique name for this machine", false),
			components.NewMulti("roles", "builder builds images, runner runs them", false),
		),
	}
}

func (a *Agents) Init() tea.Cmd {
	a.mode = agentList
	a.reload()
	return nil
}

func (a *Agents) Capturing() bool { return a.mode != agentList }

func (a *Agents) reload() {
	a.agents, _ = a.server.Store().ListAgents()
	if a.cursor >= len(a.agents) {
		a.cursor = 0
	}
	if a.mode == agentContainers || a.mode == agentLogs {
		a.loadContainers()
	}
}

func (a *Agents) loadContainers() {
	if agent := a.selected(); agent != nil {
		a.containers, _ = a.server.Store().ListContainersByAgent(agent.ID)
		if a.containerCursor >= len(a.containers) {
			a.containerCursor = 0
		}
	}
}

func (a *Agents) selected() *models.Agent {
	if a.cursor < 0 || a.cursor >= len(a.agents) {
		return nil
	}
	return &a.agents[a.cursor]
}

func (a *Agents) selectedContainer() *models.Container {
	if a.containerCursor < 0 || a.containerCursor >= len(a.containers) {
		return nil
	}
	return &a.containers[a.containerCursor]
}

func (a *Agents) Update(msg tea.Msg) tea.Cmd {
	switch message := msg.(type) {
	case ContainerLogMsg:
		if a.mode == agentLogs && message.ContainerID == a.streaming {
			a.appendLog(message)
		}
		return nil

	case tea.KeyMsg:
		return a.key(message)
	}

	a.reload()
	return nil
}

func (a *Agents) key(msg tea.KeyMsg) tea.Cmd {
	switch a.mode {
	case agentCreate:
		return a.createKey(msg)
	case agentEnrolled:
		if msg.String() == "esc" || msg.String() == "enter" {
			a.mode = agentList
			a.reload()
		}
		return nil
	case agentConfirmDelete:
		return a.deleteKey(msg)
	case agentContainers:
		return a.containerKey(msg)
	case agentLogs:
		return a.logKey(msg)
	}

	switch msg.String() {
	case "up", "k":
		a.cursor = move(a.cursor, -1, len(a.agents))
	case "down", "j":
		a.cursor = move(a.cursor, 1, len(a.agents))
	case "n":
		a.Clear()
		a.form.Reset()
		a.form.Field(1).SetOptions([]components.Option{
			{Value: string(models.RoleBuilder), Label: string(models.RoleBuilder)},
			{Value: string(models.RoleRunner), Label: string(models.RoleRunner)},
		})
		a.mode = agentCreate
	case "d":
		if a.selected() != nil {
			a.mode = agentConfirmDelete
		}
	case "enter":
		if a.selected() != nil {
			a.containerCursor = 0
			a.mode = agentContainers
			a.loadContainers()
		}
	}
	return nil
}

func (a *Agents) createKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.mode = agentList
		return nil
	case "tab", "down":
		a.form.Next()
		return nil
	case "shift+tab", "up":
		a.form.Previous()
		return nil
	case "enter":
		a.create()
		return nil
	}
	return a.form.Update(msg)
}

func (a *Agents) create() {
	if missing := a.form.Missing(); missing != "" {
		a.Notify(missing+" is required", "error")
		return
	}

	roles := make([]models.Role, 0, 2)
	for _, value := range a.form.Values(1) {
		roles = append(roles, models.Role(value))
	}

	agent := &models.Agent{
		ID:    helper.GenerateID(),
		Name:  a.form.Value(0),
		Key:   helper.GenerateToken(),
		Roles: roles,
	}

	if err := a.server.Store().CreateAgent(agent); err != nil {
		a.Notify("an agent named "+agent.Name+" already exists", "error")
		return
	}

	a.enrolled = agent
	a.mode = agentEnrolled
	a.reload()
}

func (a *Agents) deleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y":
		if agent := a.selected(); agent != nil {
			a.Fail(a.server.Store().DeleteAgent(agent.ID))
		}
		a.mode = agentList
		a.reload()
	case "n", "esc":
		a.mode = agentList
	}
	return nil
}

func (a *Agents) containerKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.mode = agentList
	case "up", "k":
		a.containerCursor = move(a.containerCursor, -1, len(a.containers))
	case "down", "j":
		a.containerCursor = move(a.containerCursor, 1, len(a.containers))
	case "enter":
		a.follow()
	}
	return nil
}

func (a *Agents) logKey(msg tea.KeyMsg) tea.Cmd {
	if msg.String() == "esc" {
		a.unfollow()
		a.mode = agentContainers
	}
	return nil
}

func (a *Agents) follow() {
	agent := a.selected()
	container := a.selectedContainer()
	if agent == nil || container == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestWindow)
	defer cancel()

	_, err := a.server.Link().Request(ctx, agent.ID, ufp.MethodLogsFollow,
		ufp.LogsFollow{ContainerID: container.ID, Tail: logTail})
	if err != nil {
		a.Fail(err)
		return
	}

	a.streaming = container.ID
	a.lines = nil
	a.mode = agentLogs
}

func (a *Agents) unfollow() {
	agent := a.selected()
	if agent == nil || a.streaming == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestWindow)
	defer cancel()

	a.server.Link().Request(ctx, agent.ID, ufp.MethodLogsStop, ufp.LogsStop{ContainerID: a.streaming})
	a.streaming = ""
}

func (a *Agents) appendLog(entry ContainerLogMsg) {
	style := theme.Body
	if entry.Stream == ufp.StreamStderr {
		style = theme.Bad
	}

	a.lines = append(a.lines, theme.Ghost.Render(time.Unix(entry.Timestamp, 0).Format("15:04:05"))+" "+
		style.Render(theme.Sanitize(entry.Line)))

	if len(a.lines) > logBuffer {
		a.lines = a.lines[len(a.lines)-logBuffer:]
	}
}

func (a *Agents) Hints() []components.Hint {
	switch a.mode {
	case agentCreate:
		return []components.Hint{{Key: "tab", Label: "next field"}, {Key: "enter", Label: "enrol"}, {Key: "esc", Label: "cancel"}}
	case agentEnrolled:
		return []components.Hint{{Key: "enter", Label: "done"}}
	case agentConfirmDelete:
		return []components.Hint{{Key: "y", Label: "remove"}, {Key: "n", Label: "keep"}}
	case agentContainers:
		return []components.Hint{{Key: "↑↓", Label: "select"}, {Key: "enter", Label: "logs"}, {Key: "esc", Label: "back"}}
	case agentLogs:
		return []components.Hint{{Key: "esc", Label: "stop following"}}
	default:
		return []components.Hint{
			{Key: "↑↓", Label: "select"},
			{Key: "enter", Label: "containers"},
			{Key: "n", Label: "enrol agent"},
			{Key: "d", Label: "remove"},
		}
	}
}

func (a *Agents) Render() string {
	switch a.mode {
	case agentCreate:
		return a.renderCreate()
	case agentEnrolled:
		return a.renderEnrolled()
	case agentConfirmDelete:
		return a.renderDelete()
	case agentContainers:
		return a.renderContainers()
	case agentLogs:
		return a.renderLogs()
	default:
		return a.renderList()
	}
}

func (a *Agents) renderList() string {
	table := components.Table{
		Columns: []components.Column{
			{Title: "agent", Width: 18},
			{Title: "roles", Width: 20},
			{Title: "state", Width: 11},
			{Title: "host", Width: 16},
			{Title: "platform", Width: 14},
			{Title: "version", Width: 9},
			{Flex: true},
			{Title: "uptime", Width: 9, Right: true},
			{Title: "seen", Width: 10, Right: true},
		},
		Cursor: a.cursor,
		Height: a.TableHeight(16),
		Empty:  "no agents enrolled yet — press n to add one",
	}

	for index := range a.agents {
		agent := &a.agents[index]
		metrics := agent.Metrics
		if metrics == nil {
			metrics = &models.Metrics{}
		}

		table.Rows = append(table.Rows, components.Row{
			components.Text(agent.Name),
			components.Roles(agent.Roles),
			components.AgentBadge(agent.Status),
			components.Styled(agent.Host, theme.Ghost),
			components.Styled(agent.Platform, theme.Ghost),
			components.Styled(agent.Version, theme.Ghost),
			components.Text(""),
			components.Styled(theme.Uptime(metrics.Uptime), theme.Ghost),
			components.Styled(theme.Since(agent.LastSeen), theme.Ghost),
		})
	}

	body := components.Card("agents", table.Render(components.CardWidth(a.Width)), a.Width, false)

	if agent := a.selected(); agent != nil && agent.Metrics != nil {
		body = components.Stack(body,
			components.Card(agent.Name, a.resources(agent), a.Width, true))
	}

	return components.Stack(body, a.Notice())
}

func (a *Agents) resources(agent *models.Agent) string {
	metrics := agent.Metrics
	return strings.Join([]string{
		components.KeyValue("cpu", meter(metrics.CPUPercent), 10),
		components.KeyValue("memory", meter(metrics.MemoryPercent)+theme.Ghost.Render(
			fmt.Sprintf("  %s / %s", theme.Bytes(metrics.MemoryUsed), theme.Bytes(metrics.MemoryTotal))), 10),
		components.KeyValue("disk", meter(metrics.DiskPercent)+theme.Ghost.Render(
			fmt.Sprintf("  %s / %s", theme.Bytes(metrics.DiskUsed), theme.Bytes(metrics.DiskTotal))), 10),
	}, "\n")
}

func (a *Agents) renderCreate() string {
	body := strings.Join([]string{
		theme.Ghost.Render("uruflow generates the id and key; you copy them onto the machine."),
		"",
		a.form.Render(components.CardWidth(a.Width)),
	}, "\n")

	return components.Stack(
		components.Card("enrol an agent", body, a.Width, true),
		a.Notice(),
	)
}

func (a *Agents) renderEnrolled() string {
	agent := a.enrolled
	if agent == nil {
		return ""
	}

	cfg := a.server.Config()
	advertise := cfg.Server.Advertise
	if advertise == "" {
		advertise = "<server-host>"
	}

	roles := make([]string, 0, len(agent.Roles))
	for _, role := range agent.Roles {
		roles = append(roles, string(role))
	}

	snippet := strings.Join([]string{
		"agent_id: " + agent.ID,
		"key: " + agent.Key,
		"roles: [" + strings.Join(roles, ", ") + "]",
		"server:",
		fmt.Sprintf("  host: %s", advertise),
		fmt.Sprintf("  port: %d", cfg.Server.UFPPort),
		"  ca_cert: /etc/uruflow/ca.crt",
	}, "\n")

	return strings.Join([]string{
		"",
		theme.Good.Render(theme.IconSuccess + " agent " + agent.Name + " enrolled"),
		"",
		theme.Heading.Render("WRITE TO /etc/uruflow/agent.yaml ON THE TARGET"),
		components.Panel("", theme.Body.Render(snippet), a.Width, false),
		"",
		theme.Heading.Render("COPY THE TRUST ROOT"),
		theme.Ghost.Render("  scp " + cfg.CACertPath() + "  <host>:/etc/uruflow/ca.crt"),
		"",
		theme.Ghost.Render("  the agent verifies both the uruflow link and the registry against this CA"),
	}, "\n")
}

func (a *Agents) renderDelete() string {
	agent := a.selected()
	if agent == nil {
		return ""
	}

	dialog := components.Dialog{
		Title:   "Remove agent",
		Message: "Remove " + agent.Name + " from uruflow?",
		Detail:  "Its containers keep running; only uruflow forgets it.",
		Confirm: "remove",
		Danger:  true,
	}
	return dialog.Render(a.Width, a.Height)
}

func (a *Agents) renderContainers() string {
	agent := a.selected()
	if agent == nil {
		return ""
	}

	table := components.Table{
		Columns: []components.Column{
			{Title: "container", Width: 22},
			{Title: "project", Width: 16},
			{Title: "service", Width: 12},
			{Title: "state", Width: 12},
			{Title: "health", Width: 10},
			{Flex: true},
			{Title: "cpu", Width: 8, Right: true},
			{Title: "memory", Width: 12, Right: true},
			{Title: "restarts", Width: 9, Right: true},
			{Title: "up", Width: 10, Right: true},
		},
		Cursor: a.containerCursor,
		Height: a.TableHeight(10),
		Empty:  "no uruflow containers on this agent",
	}

	for index := range a.containers {
		container := &a.containers[index]
		table.Rows = append(table.Rows, components.Row{
			components.Text(container.Name),
			components.Styled(container.Project, theme.Note),
			components.Styled(orDash(container.Service), theme.Mark),
			components.ContainerBadge(container.State),
			components.Health(container.Health),
			components.Text(""),
			components.Styled(fmt.Sprintf("%.1f%%", container.CPUPercent), theme.Ghost),
			components.Styled(theme.Bytes(container.MemoryUsage), theme.Ghost),
			components.Styled(fmt.Sprint(container.RestartCount), theme.Ghost),
			components.Styled(theme.Since(container.StartedAt), theme.Ghost),
		})
	}

	return components.Stack(
		components.Card(agent.Name+" containers",
			table.Render(components.CardWidth(a.Width)), a.Width, true),
		a.Notice(),
	)
}

func (a *Agents) renderLogs() string {
	container := a.selectedContainer()
	name := a.streaming
	if container != nil {
		name = container.Name
	}

	visible := a.TableHeight(6)
	lines := a.lines
	if len(lines) > visible {
		lines = lines[len(lines)-visible:]
	}
	if len(lines) == 0 {
		lines = []string{theme.Ghost.Render("attached — this container has not written anything to stdout yet")}
	}

	title := name + "  " + theme.Frame(a.Frame) + theme.Ghost.Render(" streaming")
	return components.Card(title, strings.Join(lines, "\n"), a.Width, true)
}
