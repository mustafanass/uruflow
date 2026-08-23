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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

const (
	recentReleases = 6
	fleetRows      = 6
)

type Overview struct {
	Base
	store    storage.Store
	stats    *storage.Stats
	agents   []models.Agent
	releases []models.Release
	alerts   []models.Alert
}

func NewOverview(store storage.Store) *Overview {
	return &Overview{store: store}
}

func (o *Overview) Init() tea.Cmd {
	o.reload()
	return nil
}

func (o *Overview) Update(msg tea.Msg) tea.Cmd {
	o.reload()
	return nil
}

func (o *Overview) Capturing() bool { return false }

func (o *Overview) Hints() []components.Hint {
	return []components.Hint{
		{Key: "1-6", Label: "switch view"},
		{Key: "tab", Label: "next view"},
		{Key: "?", Label: "help"},
		{Key: "q", Label: "quit"},
	}
}

func (o *Overview) reload() {
	stats, err := o.store.Stats()
	if err != nil {
		o.Fail(fmt.Errorf("load overview: %w", err))
		return
	}
	agents, err := o.store.ListAgents()
	if err != nil {
		o.Fail(fmt.Errorf("load agents: %w", err))
		return
	}
	releases, err := o.store.ListReleases(recentReleases)
	if err != nil {
		o.Fail(fmt.Errorf("load releases: %w", err))
		return
	}
	alerts, err := o.store.ListActiveAlerts()
	if err != nil {
		o.Fail(fmt.Errorf("load alerts: %w", err))
		return
	}
	o.stats, o.agents, o.releases, o.alerts = stats, agents, releases, alerts
}

func (o *Overview) Render() string {
	if o.stats == nil {
		return components.Stack(theme.Faint.Render("loading…"), o.Notice())
	}

	return components.Stack(
		components.Card("summary", o.tiles(), o.Width, false),
		components.Card("fleet", o.fleet(), o.Width, false),
		components.Card("recent releases", o.recent(), o.Width, false),
		o.Notice(),
	)
}

func (o *Overview) tiles() string {
	stats := o.stats

	agentTone := theme.Good
	if stats.AgentsOnline < stats.AgentsTotal {
		agentTone = theme.Warn
	}
	if stats.AgentsOnline == 0 {
		agentTone = theme.Bad
	}

	alertTone := theme.Good
	if stats.AlertsActive > 0 {
		alertTone = theme.Bad
	}

	rateTone := theme.Good
	if stats.SuccessRate < 80 && stats.ReleasesTotal > 0 {
		rateTone = theme.Warn
	}

	return components.StatStrip([]components.Metric{
		{Label: "agents", Value: fmt.Sprintf("%d/%d", stats.AgentsOnline, stats.AgentsTotal), Tone: agentTone},
		{Label: "builders", Value: fmt.Sprint(stats.BuildersOnline), Tone: theme.Mark},
		{Label: "runners", Value: fmt.Sprint(stats.RunnersOnline), Tone: theme.Lead},
		{Label: "projects", Value: fmt.Sprint(stats.ProjectsTotal), Tone: theme.Body},
		{Label: "releases", Value: fmt.Sprint(stats.ReleasesTotal), Tone: theme.Body},
		{Label: "today", Value: fmt.Sprint(stats.ReleasesToday), Tone: theme.Note},
		{Label: "success", Value: fmt.Sprintf("%.0f%%", stats.SuccessRate), Tone: rateTone},
		{Label: "running", Value: fmt.Sprint(stats.ContainersRunning), Tone: theme.Good},
		{Label: "alerts", Value: fmt.Sprint(stats.AlertsActive), Tone: alertTone},
	}, components.CardWidth(o.Width))
}

func (o *Overview) fleet() string {
	table := components.Table{
		Columns: []components.Column{
			{Title: "agent", Width: 18},
			{Title: "roles", Width: 20},
			{Title: "state", Width: 11},
			{Title: "cpu", Width: 16},
			{Title: "memory", Width: 16},
			{Title: "disk", Width: 16},
			{Flex: true},
			{Title: "seen", Width: 10, Right: true},
		},
		Cursor: -1,
		Height: fleetRows,
		Empty:  "no agents enrolled — press 3 then n to add one",
	}

	for _, agent := range o.agents {
		metrics := agent.Metrics
		if metrics == nil {
			metrics = &models.Metrics{}
		}
		table.Rows = append(table.Rows, components.Row{
			components.Text(agent.Name),
			components.Roles(agent.Roles),
			components.AgentBadge(agent.Status),
			components.Text(meter(metrics.CPUPercent)),
			components.Text(meter(metrics.MemoryPercent)),
			components.Text(meter(metrics.DiskPercent)),
			components.Text(""),
			components.Styled(theme.Since(agent.LastSeen), theme.Ghost),
		})
	}

	return table.Render(components.CardWidth(o.Width))
}

func (o *Overview) recent() string {
	table := components.Table{
		Columns: []components.Column{
			{Title: "project", Width: 16},
			{Title: "pipeline", Width: 30},
			{Title: "commit", Width: 9},
			{Title: "image", Width: 18, Flex: true},
			{Title: "trigger", Width: 10},
			{Title: "took", Width: 8, Right: true},
			{Title: "when", Width: 10, Right: true},
		},
		Cursor: -1,
		Height: recentReleases,
		Empty:  "no releases yet — press 2 to add a project",
	}

	for index := range o.releases {
		release := &o.releases[index]
		table.Rows = append(table.Rows, components.Row{
			components.Text(release.Project),
			components.Text(components.Steps(components.ReleaseSteps(release))),
			components.Styled(theme.ShortSHA(release.Commit), theme.Ghost),
			components.Styled(theme.ImageTag(release.Image), theme.Note),
			components.Styled(string(release.Trigger), theme.Ghost),
			components.Styled(theme.Duration(release.Duration), theme.Ghost),
			components.Styled(theme.Since(release.StartedAt), theme.Ghost),
		})
	}

	return table.Render(components.CardWidth(o.Width))
}

func meter(percent float64) string {
	return components.Gauge(percent, 8) + theme.Faint.Render(fmt.Sprintf(" %3.0f%%", percent))
}
