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
	"crypto/subtle"

	"github.com/gin-gonic/gin"
	"github.com/mustafanass/uruflow/internal/storage"
)

func currentUser(c *gin.Context) *storage.ConsoleUser {
	return c.MustGet("user").(*storage.ConsoleUser)
}

func (h *Handler) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, user, err := h.auth.identify(c.Request)
		if err != nil {
			writeError(c.Writer, 401, "Sign in to continue.")
			c.Abort()
			return
		}
		if rank(user.Role) < rank("viewer") {
			writeError(c.Writer, 403, "Your role does not allow this action.")
			c.Abort()
			return
		}
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(session.CSRF)) != 1 {
			writeError(c.Writer, 403, "The session verification token is invalid.")
			c.Abort()
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

func (h *Handler) requireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rank(currentUser(c).Role) < rank(role) {
			writeError(c.Writer, 403, "Your role does not allow this action.")
			c.Abort()
			return
		}
		c.Next()
	}
}
