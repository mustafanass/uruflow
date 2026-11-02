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

func (h *Handler) images(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	if !requireFleet(w, u) {
		return
	}
	rows, err := h.operations.Images(r.Context())
	if err != nil {
		writeError(w, 503, "Registry catalog unavailable.")
		return
	}
	writeJSON(w, 200, rows)
}

func (h *Handler) removeImage(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct{ Repository, Tag string }
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "image.remove", body.Repository+":"+body.Tag, func() (any, error) {
		return map[string]bool{"removed": true}, h.operations.RemoveImage(r.Context(), body.Repository, body.Tag)
	})
}
