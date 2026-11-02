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
)

func (h *Handler) enrollAgent(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct {
		Name  string
		Roles []string
	}
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "agent.enroll", body.Name, func() (any, error) { return h.operations.EnrollAgent(body.Name, body.Roles) })
}

func (h *Handler) removeAgent(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	name := c.Param("name")
	h.mutation(w, r, u, "agent.remove", name, func() (any, error) { return map[string]bool{"removed": true}, h.operations.RemoveAgent(name) })
}

func (h *Handler) trustRoot(c *gin.Context) {
	w := c.Writer
	data, err := h.operations.TrustRoot()
	if err != nil {
		writeError(w, 503, "Trust root unavailable.")
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="uruflow-ca.crt"`)
	_, _ = w.Write(data)
}
