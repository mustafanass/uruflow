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
	"github.com/gin-gonic/gin"
	"github.com/mustafanass/uruflow/internal/models"
)

func (h *Handler) alerts(c *gin.Context) {
	u := currentUser(c)
	rows, err := h.operations.RecentAlerts()
	if err != nil {
		writeError(c.Writer, 500, "Could not load alert history.")
		return
	}
	visible := make([]models.Alert, 0, len(rows))
	for _, alert := range rows {
		if u.AllProjects() || (alert.Project != "" && u.CanProject(alert.Project)) {
			visible = append(visible, alert)
		}
	}
	writeJSON(c.Writer, 200, visible)
}

func (h *Handler) snapshot(c *gin.Context) {
	w := c.Writer
	u := currentUser(c)
	h.mu.Lock()
	snapshot := h.latest
	h.mu.Unlock()
	if snapshot == nil {
		writeError(w, 503, "Waiting for the first observation.")
		return
	}
	writeJSON(w, 200, visibleSnapshot(*snapshot, u))
}

func (h *Handler) resolveAlert(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	id := c.Param("id")
	alert, err := h.operations.Alert(id)
	if err != nil || (!u.AllProjects() && (alert.Project == "" || !u.CanProject(alert.Project))) {
		writeError(w, 404, "Alert not found.")
		return
	}
	h.mutation(w, r, u, "alert.resolve", id, func() (any, error) { return map[string]bool{"resolved": true}, h.operations.ResolveAlert(id) })
}

func (h *Handler) audit(c *gin.Context) {
	w := c.Writer
	rows, err := h.accounts.ListAudit(200)
	if err != nil {
		writeError(w, 500, "Could not load audit records.")
		return
	}
	writeJSON(w, 200, rows)
}
