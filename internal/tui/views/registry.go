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
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/registry"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

const (
	catalogTimeout = 20 * time.Second
	catalogRefresh = 15 * time.Second
)

type Registry struct {
	Base
	server   *api.Server
	cursor   int
	images   []models.Image
	healthy  bool
	loadedAt time.Time
	confirm  bool
}

func NewRegistry(server *api.Server) *Registry {
	return &Registry{server: server}
}

func (r *Registry) Init() tea.Cmd {
	r.load()
	return nil
}

func (r *Registry) Capturing() bool { return r.confirm }

func (r *Registry) load() {
	ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
	defer cancel()

	r.healthy = r.server.Registry().Health(ctx) == nil
	if !r.healthy {
		r.images = nil
		return
	}

	images, err := r.server.Registry().Images(ctx)
	if err != nil {
		r.Fail(err)
		return
	}

	r.images = images
	r.loadedAt = time.Now()
	if r.cursor >= len(r.images) {
		r.cursor = 0
	}
}

func (r *Registry) selected() *models.Image {
	if r.cursor < 0 || r.cursor >= len(r.images) {
		return nil
	}
	return &r.images[r.cursor]
}

func (r *Registry) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok {
		return r.key(key)
	}

	if time.Since(r.loadedAt) > catalogRefresh {
		r.load()
	}
	return nil
}

func (r *Registry) key(msg tea.KeyMsg) tea.Cmd {
	if r.confirm {
		switch msg.String() {
		case "y":
			r.remove()
			r.confirm = false
		case "n", "esc":
			r.confirm = false
		}
		return nil
	}

	switch msg.String() {
	case "up", "k":
		r.cursor = move(r.cursor, -1, len(r.images))
	case "down", "j":
		r.cursor = move(r.cursor, 1, len(r.images))
	case "r":
		r.Clear()
		r.load()
	case "d":
		if r.selected() != nil {
			r.confirm = true
		}
	}
	return nil
}

func (r *Registry) remove() {
	image := r.selected()
	if image == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
	defer cancel()

	if err := r.server.Registry().DeleteTag(ctx, image.Repository, image.Tag); err != nil {
		r.Fail(err)
		return
	}

	r.Notify("deleted "+image.Repository+":"+image.Tag, "success")
	r.load()
}

func (r *Registry) Hints() []components.Hint {
	if r.confirm {
		return []components.Hint{{Key: "y", Label: "delete"}, {Key: "n", Label: "cancel"}}
	}
	return []components.Hint{
		{Key: "↑↓", Label: "select"},
		{Key: "r", Label: "refresh"},
		{Key: "d", Label: "delete tag"},
	}
}

func (r *Registry) Render() string {
	if r.confirm {
		image := r.selected()
		dialog := components.Dialog{
			Title:   "Delete image tag",
			Message: "Delete " + image.Repository + ":" + image.Tag + " from the registry?",
			Detail:  "Runners already using it keep running; new pulls will fail.",
			Confirm: "delete",
			Danger:  true,
		}
		return dialog.Render(r.Width, r.Height)
	}

	table := components.Table{
		Columns: []components.Column{
			{Title: "repository", Width: 26, Flex: true},
			{Title: "tag", Width: 16},
			{Title: "digest", Width: 16},
			{Title: "size", Width: 10, Right: true},
		},
		Cursor: r.cursor,
		Height: r.TableHeight(12),
		Empty:  "the registry holds no images yet",
	}

	for index := range r.images {
		image := &r.images[index]
		table.Rows = append(table.Rows, components.Row{
			components.Text(image.Repository),
			components.Styled(image.Tag, theme.Note),
			components.Styled(registry.ShortDigest(image.Digest), theme.Ghost),
			components.Styled(theme.Bytes(uint64(image.Size)), theme.Ghost),
		})
	}

	return components.Stack(
		components.Card("bundled registry", r.summary(), r.Width, false),
		components.Card("images", table.Render(components.CardWidth(r.Width)), r.Width, false),
		r.Notice(),
	)
}

func (r *Registry) summary() string {
	options := r.server.Registry().Options()

	state := theme.Bad.Render(theme.IconFailure + " unreachable")
	if r.healthy {
		state = theme.Good.Render(theme.IconOnline + " healthy")
	}

	return strings.Join([]string{
		components.KeyValue("endpoint", theme.Note.Render("https://"+options.Address), 12),
		components.KeyValue("namespace", options.Namespace, 12),
		components.KeyValue("state", state, 12),
	}, "\n")
}

func (r *Registry) Healthy() bool { return r.healthy }
