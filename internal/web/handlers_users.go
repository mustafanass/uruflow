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
	"github.com/mustafanass/uruflow/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) users(c *gin.Context) {
	w := c.Writer
	users, err := h.accounts.ListConsoleUsers()
	if err != nil {
		writeError(w, 500, "Could not load users.")
		return
	}
	writeJSON(w, 200, users)
}

func (h *Handler) createUser(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct {
		Username, Password, Role string
		Projects                 []string
	}
	if !readJSON(w, r, &body) {
		return
	}
	projects, valid := validProjects(body.Projects)
	if !validCredentials(body.Username, body.Password) || rank(body.Role) == 0 || !valid {
		writeError(w, 400, "Choose a valid username, role and password of at least 15 characters (maximum 72 bytes).")
		return
	}
	h.mutation(w, r, u, "user.create", body.Username, func() (any, error) {
		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		return map[string]bool{"created": true}, h.accounts.CreateConsoleUser(&storage.ConsoleUser{Username: body.Username, Role: body.Role, Projects: projects, PasswordHash: hash})
	})
}

func (h *Handler) removeUser(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	name := c.Param("name")
	if name == u.Username {
		writeError(w, 400, "You cannot remove your current account.")
		return
	}
	h.mutation(w, r, u, "user.remove", name, func() (any, error) {
		if err := h.accounts.DeleteConsoleUser(name); err != nil {
			return nil, err
		}
		if err := h.auth.revokeUser(name); err != nil {
			return nil, err
		}
		return map[string]bool{"removed": true}, nil
	})
}

func (h *Handler) changeRole(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct{ Role string }
	if !readJSON(w, r, &body) {
		return
	}
	if rank(body.Role) == 0 {
		writeError(w, 400, "Invalid role.")
		return
	}
	name := c.Param("name")
	h.mutation(w, r, u, "user.role.change", name, func() (any, error) {
		if err := h.accounts.ChangeConsoleRole(name, body.Role); err != nil {
			return nil, err
		}
		if err := h.auth.revokeUser(name); err != nil {
			return nil, err
		}
		return map[string]bool{"changed": true}, nil
	})
}

func (h *Handler) changeProjects(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct{ Projects []string }
	if !readJSON(w, r, &body) {
		return
	}
	projects, ok := validProjects(body.Projects)
	if !ok {
		writeError(w, 400, "Invalid project assignments.")
		return
	}
	name := c.Param("name")
	h.mutation(w, r, u, "user.projects.change", name, func() (any, error) {
		if err := h.accounts.ChangeConsoleProjects(name, projects); err != nil {
			return nil, err
		}
		if err := h.auth.revokeUser(name); err != nil {
			return nil, err
		}
		return map[string]bool{"changed": true}, nil
	})
}

func (h *Handler) resetPassword(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct{ Password string }
	if !readJSON(w, r, &body) {
		return
	}
	name := c.Param("name")
	if name == u.Username {
		writeError(w, 400, "Use Change password for your own account.")
		return
	}
	if !validPassword(name, body.Password) {
		writeError(w, 400, "Use a password of at least 15 characters (maximum 72 bytes).")
		return
	}
	h.mutation(w, r, u, "user.password.reset", name, func() (any, error) {
		target, err := h.accounts.GetConsoleUser(name)
		if err != nil {
			return nil, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		if err = h.accounts.ChangeConsolePassword(name, target.PasswordHash, hash); err != nil {
			return nil, err
		}
		if err := h.auth.revokeUser(name); err != nil {
			return nil, err
		}
		return map[string]bool{"changed": true}, nil
	})
}

func (h *Handler) revokeSessions(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	h.mutation(w, r, u, "user.sessions.revoke", u.Username, func() (any, error) {
		if err := h.auth.revokeUser(u.Username); err != nil {
			return nil, err
		}
		return map[string]bool{"revoked": true}, nil
	})
}

func (h *Handler) changePassword(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !validPassword(u.Username, body.NewPassword) {
		writeError(w, 400, "Use a password of at least 15 characters (maximum 72 bytes).")
		return
	}
	if bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(body.CurrentPassword)) != nil {
		writeError(w, 403, "The current password is incorrect.")
		return
	}
	h.mutation(w, r, u, "user.password.change", u.Username, func() (any, error) {
		hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		if err = h.accounts.ChangeConsolePassword(u.Username, u.PasswordHash, hash); err != nil {
			return nil, err
		}
		if err := h.auth.revokeUser(u.Username); err != nil {
			return nil, err
		}
		return map[string]bool{"changed": true}, nil
	})
}
