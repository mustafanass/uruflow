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
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

const releaseHistory = 60

type releaseMode int

const (
	releaseList releaseMode = iota
	releaseDetail
)

type Releases struct {
	Base
	server   *api.Server
	mode     releaseMode
	cursor   int
	releases []models.Release
	current  *models.Release
	logs     []models.LogLine
	follow   bool
}

func NewReleases(server *api.Server) *Releases {
	return &Releases{server: server, follow: true}
}

func (r *Releases) Init() tea.Cmd {
	r.mode = releaseList
	r.reload()
	return nil
}

func (r *Releases) Capturing() bool { return r.mode == releaseDetail }

func (r *Releases) reload() {
	releases, err := r.server.Store().ListReleases(releaseHistory)
	if err != nil {
		r.Fail(fmt.Errorf("load releases: %w", err))
		return
	}
	r.releases = releases
	if r.cursor >= len(r.releases) {
		r.cursor = 0
	}

	if r.mode == releaseDetail && r.current != nil {
		refreshed, err := r.server.Store().GetRelease(r.current.ID)
		if err != nil {
			r.Fail(fmt.Errorf("load release %s: %w", r.current.ID, err))
			return
		}
		r.current = refreshed
		logs, err := r.server.Store().ListLogs(r.current.ID)
		if err != nil {
			r.Fail(fmt.Errorf("load release logs: %w", err))
			return
		}
		r.logs = logs
	}
}

func (r *Releases) selected() *models.Release {
	if r.cursor < 0 || r.cursor >= len(r.releases) {
		return nil
	}
	return &r.releases[r.cursor]
}

func (r *Releases) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok {
		return r.key(key)
	}
	r.reload()
	return nil
}

func (r *Releases) key(msg tea.KeyMsg) tea.Cmd {
	if r.mode == releaseDetail {
		switch msg.String() {
		case "esc":
			r.mode = releaseList
		case "f":
			r.follow = !r.follow
		}
		return nil
	}

	switch msg.String() {
	case "up", "k":
		r.cursor = move(r.cursor, -1, len(r.releases))
	case "down", "j":
		r.cursor = move(r.cursor, 1, len(r.releases))
	case "enter":
		if release := r.selected(); release != nil {
			current, err := r.server.Store().GetRelease(release.ID)
			if err != nil {
				r.Fail(fmt.Errorf("load release %s: %w", release.ID, err))
				return nil
			}
			r.current = current
			r.follow = true
			r.mode = releaseDetail
			r.reload()
		}
	}
	return nil
}

func (r *Releases) Hints() []components.Hint {
	if r.mode == releaseDetail {
		return []components.Hint{
			{Key: "f", Label: "toggle follow"},
			{Key: "esc", Label: "back"},
		}
	}
	return []components.Hint{
		{Key: "↑↓", Label: "select"},
		{Key: "enter", Label: "open"},
	}
}

func (r *Releases) Render() string {
	if r.mode == releaseDetail {
		return r.renderDetail()
	}
	return r.renderList()
}

func (r *Releases) renderList() string {
	table := components.Table{
		Columns: []components.Column{
			{Title: "project", Width: 16},
			{Title: "pipeline", Width: 30},
			{Title: "status", Width: 13},
			{Title: "commit", Width: 9},
			{Title: "image", Width: 18, Flex: true},
			{Title: "builder", Width: 14},
			{Title: "trigger", Width: 10},
			{Title: "took", Width: 8, Right: true},
			{Title: "when", Width: 10, Right: true},
		},
		Cursor: r.cursor,
		Height: r.TableHeight(8),
		Empty:  "no releases yet — deploy a project from the projects view",
	}

	for index := range r.releases {
		release := &r.releases[index]
		table.Rows = append(table.Rows, components.Row{
			components.Text(release.Project),
			components.Text(components.Steps(components.ReleaseSteps(release))),
			components.StatusBadge(release.Status),
			components.Styled(theme.ShortSHA(release.Commit), theme.Ghost),
			components.Styled(theme.ImageTag(release.Image), theme.Note),
			components.Styled(release.BuilderName, theme.Mark),
			components.Styled(string(release.Trigger), theme.Ghost),
			components.Styled(theme.Duration(release.Duration), theme.Ghost),
			components.Styled(theme.Since(release.StartedAt), theme.Ghost),
		})
	}

	return components.Stack(
		components.Card("releases", table.Render(components.CardWidth(r.Width)), r.Width, false),
		r.Notice(),
	)
}

func (r *Releases) renderDetail() string {
	release := r.current
	if release == nil {
		return ""
	}

	summary := []string{
		components.KeyValue("status", components.StatusBadge(release.Status).Text, 11),
		components.KeyValue("pipeline", components.Steps(components.ReleaseSteps(release)), 11),
		components.KeyValue("image", theme.Note.Render(orDash(release.Image)), 11),
		components.KeyValue("commit", theme.Ghost.Render(orDash(theme.ShortSHA(release.Commit)))+
			theme.Ghost.Render("  branch "+release.Branch), 11),
		components.KeyValue("builder", theme.Mark.Render(orDash(release.BuilderName))+
			theme.Ghost.Render("  trigger "+string(release.Trigger)), 11),
	}
	if release.Digest != "" {
		summary = append(summary, components.KeyValue("digest", theme.Ghost.Render(release.Digest), 11))
	}
	if release.Message != "" {
		summary = append(summary, components.KeyValue("message", theme.Bad.Render(release.Message), 11))
	}

	return components.Stack(
		components.Card(release.Project+"  "+release.ID, strings.Join(summary, "\n"), r.Width, true),
		components.Card("runners", r.renderTargets(release), r.Width, false),
		r.renderLogs(),
	)
}

func (r *Releases) renderTargets(release *models.Release) string {
	if len(release.Targets) == 0 {
		return theme.Ghost.Render("none")
	}

	lines := make([]string, 0, len(release.Targets))
	for _, target := range release.Targets {
		line := components.StatusStyle(target.Status).Render(components.StatusIcon(target.Status)) +
			" " + theme.Cell(target.AgentName, 18) +
			components.StatusStyle(target.Status).Render(theme.Cell(string(target.Status), 12))
		if target.Message != "" {
			line += theme.Ghost.Render(target.Message)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (r *Releases) renderLogs() string {
	label := "logs"
	if r.follow {
		label += "  " + theme.Frame(r.Frame) + theme.Ghost.Render(" following")
	}

	if len(r.logs) == 0 {
		return components.Card(label, theme.Ghost.Render("no output yet"), r.Width, false)
	}

	visible := r.TableHeight(24)
	entries := r.logs
	if r.follow && len(entries) > visible {
		entries = entries[len(entries)-visible:]
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		style := theme.Body
		if entry.Stream == "stderr" {
			style = theme.Bad
		}

		stage := theme.Mark
		if entry.Stage == "release" {
			stage = theme.Lead
		}

		lines = append(lines,
			stage.Render(theme.Cell(entry.Stage, 8))+
				theme.Ghost.Render(theme.Cell(entry.AgentName, 14))+
				style.Render(theme.Truncate(theme.Sanitize(entry.Line), r.Width-30)))
	}

	return components.Card(label, strings.Join(lines, "\n"), r.Width, false)
}

func orDash(value string) string {
	if value == "" {
		return "–"
	}
	return value
}
