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

package web

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

type terminalOwner struct {
	username, project, agentID, containerID string
}

func (h *Handler) projectContainer(c *gin.Context) (*models.Container, bool) {
	u := currentUser(c)
	container, err := h.operations.Container(c.Param("agent"), c.Param("id"))
	if err != nil || container.Project == "" || !u.CanProject(container.Project) {
		writeError(c.Writer, 404, "Container not found.")
		return nil, false
	}
	return container, true
}

func (h *Handler) restartContainer(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	container, ok := h.projectContainer(c)
	if !ok {
		return
	}
	h.mutation(w, r, u, "container.restart", container.Project+"/"+container.Name, func() (any, error) {
		return map[string]bool{"restarted": true}, h.operations.RestartContainer(c.Param("agent"), c.Param("id"))
	})
}

func (h *Handler) containerTerminal(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	container, ok := h.projectContainer(c)
	if !ok {
		return
	}
	columns, _ := strconv.Atoi(r.URL.Query().Get("columns"))
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	if !ufp.ValidTerminalSize(columns, rows) {
		writeError(w, 400, "Invalid terminal size.")
		return
	}
	agentID := c.Param("agent")
	containerID := c.Param("id")
	resource := container.Project + "/" + container.Name
	h.logStream(w, r, u, func(ctx context.Context, emit func(logEvent) error) error {
		if err := h.record(u.Username, "container.terminal", resource, "requested"); err != nil {
			return errors.New("audit storage is unavailable; the terminal was not opened")
		}
		session, err := randomToken()
		if err != nil {
			return err
		}
		h.mu.Lock()
		h.terminals[session] = terminalOwner{username: u.Username, project: container.Project, agentID: agentID, containerID: containerID}
		h.mu.Unlock()
		err = emit(logEvent{kind: "session", value: map[string]string{"id": session}})
		if err == nil {
			err = h.operations.OpenTerminal(ctx, agentID, containerID, session, columns, rows, func(output ufp.TerminalOutput) error {
				return emit(logEvent{kind: "output", value: output})
			})
		}
		h.mu.Lock()
		delete(h.terminals, session)
		h.mu.Unlock()
		outcome := "succeeded"
		if err != nil && ctx.Err() == nil {
			outcome = "failed"
		}
		_ = h.record(u.Username, "container.terminal", resource, outcome)
		return err
	})
}

func (h *Handler) terminalInput(c *gin.Context) {
	w := c.Writer
	r := c.Request
	var body struct {
		Data string `json:"data"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Data == "" || len(body.Data) > ufp.MaxTerminalInput {
		writeError(w, 400, "Invalid terminal input.")
		return
	}
	owner, ok := h.terminalSession(c)
	if !ok {
		return
	}
	if err := h.operations.TerminalInput(owner.agentID, c.Param("session"), []byte(body.Data)); err != nil {
		writeError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"accepted": true})
}

func (h *Handler) resizeTerminal(c *gin.Context) {
	w := c.Writer
	r := c.Request
	var body struct {
		Columns int `json:"columns"`
		Rows    int `json:"rows"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !ufp.ValidTerminalSize(body.Columns, body.Rows) {
		writeError(w, 400, "Invalid terminal size.")
		return
	}
	owner, ok := h.terminalSession(c)
	if !ok {
		return
	}
	if err := h.operations.ResizeTerminal(owner.agentID, c.Param("session"), body.Columns, body.Rows); err != nil {
		writeError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"accepted": true})
}

func (h *Handler) terminalSession(c *gin.Context) (terminalOwner, bool) {
	u := currentUser(c)
	h.mu.Lock()
	owner, found := h.terminals[c.Param("session")]
	h.mu.Unlock()
	if !found || owner.username != u.Username || owner.agentID != c.Param("agent") || owner.containerID != c.Param("id") || !u.CanProject(owner.project) {
		writeError(c.Writer, 404, "Terminal session not found.")
		return terminalOwner{}, false
	}
	return owner, true
}
