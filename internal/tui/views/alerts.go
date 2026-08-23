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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

const alertHistory = 80

type Alerts struct {
	Base
	store    storage.Store
	cursor   int
	alerts   []models.Alert
	resolved bool
}

func NewAlerts(store storage.Store) *Alerts {
	return &Alerts{store: store}
}

func (a *Alerts) Init() tea.Cmd {
	a.reload()
	return nil
}

func (a *Alerts) Capturing() bool { return false }

func (a *Alerts) reload() {
	var alerts []models.Alert
	var err error
	if a.resolved {
		alerts, err = a.store.ListRecentAlerts(alertHistory)
	} else {
		alerts, err = a.store.ListActiveAlerts()
	}
	if err != nil {
		a.Fail(err)
		return
	}
	a.alerts = alerts
	if a.cursor >= len(a.alerts) {
		a.cursor = 0
	}
}

func (a *Alerts) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			a.cursor = move(a.cursor, -1, len(a.alerts))
		case "down", "j":
			a.cursor = move(a.cursor, 1, len(a.alerts))
		case "a":
			a.resolved = !a.resolved
			a.cursor = 0
			a.reload()
		case "r":
			if a.cursor < len(a.alerts) {
				a.Fail(a.store.ResolveAlert(a.alerts[a.cursor].ID))
				a.reload()
			}
		}
		return nil
	}

	a.reload()
	return nil
}

func (a *Alerts) Hints() []components.Hint {
	scope := "show all"
	if a.resolved {
		scope = "show active only"
	}
	return []components.Hint{
		{Key: "↑↓", Label: "select"},
		{Key: "r", Label: "resolve"},
		{Key: "a", Label: scope},
	}
}

func (a *Alerts) Render() string {
	table := components.Table{
		Columns: []components.Column{
			{Title: "severity", Width: 12},
			{Title: "agent", Width: 18},
			{Title: "type", Width: 16},
			{Title: "message", Width: 30, Flex: true},
			{Title: "state", Width: 10},
			{Title: "raised", Width: 10, Right: true},
		},
		Cursor: a.cursor,
		Height: a.TableHeight(8),
		Empty:  "nothing is wrong right now",
	}

	for index := range a.alerts {
		alert := &a.alerts[index]

		state := components.Styled("active", theme.Warn)
		if alert.Resolved {
			state = components.Styled("resolved", theme.Ghost)
		}

		table.Rows = append(table.Rows, components.Row{
			components.SeverityBadge(alert.Severity),
			components.Text(alert.AgentName),
			components.Styled(alert.Type, theme.Ghost),
			components.Text(alert.Message),
			state,
			components.Styled(theme.Since(alert.CreatedAt), theme.Ghost),
		})
	}

	scope := "active alerts"
	if a.resolved {
		scope = "all alerts"
	}

	return components.Stack(
		components.Card(scope, table.Render(components.CardWidth(a.Width)), a.Width, false),
		a.Notice(),
	)
}
