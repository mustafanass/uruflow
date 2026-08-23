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
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/secrets"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

type secretMode int

const (
	secretList secretMode = iota
	secretCreate
	secretConfirmDelete
)

const (
	secretFieldName = iota
	secretFieldValue
)

type Secrets struct {
	Base
	server  *api.Server
	mode    secretMode
	cursor  int
	secrets []models.Secret
	form    *components.Form
}

func NewSecrets(server *api.Server) *Secrets {
	value := components.NewField("value", "", "stored encrypted; never shown again", false)
	value.Secret()

	return &Secrets{
		server: server,
		form: components.NewForm(
			components.NewField("name", "api_db_url", "referenced as ${secret:name}", false),
			value,
		),
	}
}

func (s *Secrets) Init() tea.Cmd {
	s.mode = secretList
	s.reload()
	return nil
}

func (s *Secrets) Capturing() bool { return s.mode != secretList }

func (s *Secrets) reload() {
	secretsList, err := s.server.Store().ListSecrets()
	if err != nil {
		s.Fail(err)
		return
	}
	s.secrets = secretsList
	if s.cursor >= len(s.secrets) {
		s.cursor = 0
	}
}

func (s *Secrets) selected() *models.Secret {
	if s.cursor < 0 || s.cursor >= len(s.secrets) {
		return nil
	}
	return &s.secrets[s.cursor]
}

func (s *Secrets) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok {
		return s.key(key)
	}
	s.reload()
	return nil
}

func (s *Secrets) key(msg tea.KeyMsg) tea.Cmd {
	switch s.mode {
	case secretCreate:
		return s.createKey(msg)
	case secretConfirmDelete:
		switch msg.String() {
		case "y":
			if secret := s.selected(); secret != nil {
				s.Fail(s.server.Store().DeleteSecret(secret.Name))
			}
			s.mode = secretList
			s.reload()
		case "n", "esc":
			s.mode = secretList
		}
		return nil
	}

	switch msg.String() {
	case "up", "k":
		s.cursor = move(s.cursor, -1, len(s.secrets))
	case "down", "j":
		s.cursor = move(s.cursor, 1, len(s.secrets))
	case "n":
		s.Clear()
		s.form.Reset()
		s.mode = secretCreate
	case "d":
		if s.selected() != nil {
			s.mode = secretConfirmDelete
		}
	}
	return nil
}

func (s *Secrets) createKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		s.form.Reset()
		s.mode = secretList
		return nil
	case "tab", "down":
		s.form.Next()
		return nil
	case "shift+tab", "up":
		s.form.Previous()
		return nil
	case "enter":
		s.create()
		return nil
	}
	return s.form.Update(msg)
}

func (s *Secrets) create() {
	if missing := s.form.Missing(); missing != "" {
		s.Notify(missing+" is required", "error")
		return
	}

	name := s.form.Value(secretFieldName)
	sealed, err := s.server.Vault().Seal(s.form.Value(secretFieldValue))
	if err != nil {
		s.Fail(err)
		return
	}
	if err := s.server.Store().SetSecret(name, sealed); err != nil {
		s.Fail(err)
		return
	}

	s.form.Reset()
	s.mode = secretList
	s.reload()
	s.Notify("stored "+name+" — reference it as ${secret:"+name+"}", "success")
}

func (s *Secrets) Hints() []components.Hint {
	switch s.mode {
	case secretCreate:
		return []components.Hint{
			{Key: "tab", Label: "next field"},
			{Key: "enter", Label: "store"},
			{Key: "esc", Label: "cancel"},
		}
	case secretConfirmDelete:
		return []components.Hint{{Key: "y", Label: "remove"}, {Key: "n", Label: "keep"}}
	default:
		return []components.Hint{
			{Key: "↑↓", Label: "select"},
			{Key: "n", Label: "new secret"},
			{Key: "d", Label: "remove"},
		}
	}
}

func (s *Secrets) Render() string {
	switch s.mode {
	case secretCreate:
		body := strings.Join([]string{
			theme.Ghost.Render("the value is encrypted with pki/secrets.key and never displayed again"),
			"",
			s.form.Render(components.CardWidth(s.Width)),
		}, "\n")
		return components.Stack(components.Card("new secret", body, s.Width, true), s.Notice())

	case secretConfirmDelete:
		secret := s.selected()
		if secret == nil {
			return ""
		}
		dialog := components.Dialog{
			Title:   "Remove secret",
			Message: "Remove " + secret.Name + "?",
			Detail:  "Projects referencing it will fail to deploy until it is set again.",
			Confirm: "remove",
			Danger:  true,
		}
		return dialog.Render(s.Width, s.Height)
	}

	table := components.Table{
		Columns: []components.Column{
			{Title: "name", Width: 28},
			{Title: "value", Width: 14},
			{Title: "reference", Width: 30, Flex: true},
			{Title: "updated", Width: 12, Right: true},
		},
		Cursor: s.cursor,
		Height: s.TableHeight(10),
		Empty:  "no secrets stored — press n to add one",
	}

	for index := range s.secrets {
		secret := &s.secrets[index]
		table.Rows = append(table.Rows, components.Row{
			components.Text(secret.Name),
			components.Styled(secrets.Masked, theme.Ghost),
			components.Styled("${secret:"+secret.Name+"}", theme.Note),
			components.Styled(theme.Since(secret.UpdatedAt), theme.Ghost),
		})
	}

	return components.Stack(
		components.Card("secrets", table.Render(components.CardWidth(s.Width)), s.Width, false),
		components.Card("how to use", strings.Join([]string{
			theme.Ghost.Render("put the reference in a project variable, not the value:"),
			"  " + theme.Body.Render("DATABASE_URL=") + theme.Note.Render("${secret:api_db_url}"),
			"",
			theme.Ghost.Render("uruflow resolves it when a release is dispatched, so the value never"),
			theme.Ghost.Render("appears in a project file, in this interface, or in a release log"),
		}, "\n"), s.Width, false),
		s.Notice(),
	)
}
