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
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *Handler) createProject(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	if !requireFleet(w, u) {
		return
	}
	var body struct{ Name, Environment, Content string }
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "project.create", body.Name+"/"+body.Environment, func() (any, error) { return h.operations.CreateProject(body.Name, body.Environment, body.Content) })
}

func (h *Handler) reloadProjects(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	if !requireFleet(w, u) {
		return
	}
	h.mutation(w, r, u, "project.reload", "projects", func() (any, error) { return map[string]any{"problems": h.operations.ReloadProjects()}, nil })
}

func (h *Handler) projectReleases(c *gin.Context) {
	w := c.Writer
	u := currentUser(c)
	if !requireProject(w, u, c.Param("name")) {
		return
	}
	rows, err := h.operations.ProjectReleases(c.Param("name"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

func (h *Handler) getProjectDocument(c *gin.Context) {
	w := c.Writer
	u := currentUser(c)
	kind := "configuration"
	if strings.HasSuffix(c.FullPath(), "/variables") {
		kind = "variables"
	}
	variables := kind == "variables"
	if !requireProject(w, u, c.Param("name")) {
		return
	}
	doc, err := h.operations.ProjectDocument(c.Param("name"), variables)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, doc)
}

func (h *Handler) saveProjectDocument(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	kind := "configuration"
	if strings.HasSuffix(c.FullPath(), "/variables") {
		kind = "variables"
	}
	variables := kind == "variables"
	if !requireProject(w, u, c.Param("name")) {
		return
	}
	var body struct{ Content, Revision string }
	if !readJSON(w, r, &body) {
		return
	}
	name := c.Param("name")
	policy, err := h.documentPolicy(u, name)
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	h.mutation(w, r, u, "project."+kind+".save", name, func() (any, error) {
		return h.operations.SaveProjectDocument(name, variables, body.Content, body.Revision, policy)
	})
}

func (h *Handler) getPushSettings(c *gin.Context) {
	w := c.Writer
	u := currentUser(c)
	if !requireProject(w, u, c.Param("name")) {
		return
	}
	settings, err := h.operations.PushSettings(c.Param("name"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, settings)
}

func (h *Handler) setPushSource(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	name := c.Param("name")
	if !requireProject(w, u, name) {
		return
	}
	var body struct {
		Repository string `json:"repository"`
		Branch     string `json:"branch"`
		Enabled    bool   `json:"enabled"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "project.push.update", name, func() (any, error) {
		return h.operations.SetPushSource(name, body.Repository, body.Branch, body.Enabled)
	})
}

func (h *Handler) projectAction(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	action := c.FullPath()[strings.LastIndex(c.FullPath(), "/")+1:]
	name := c.Param("name")
	if !requireProject(w, u, name) {
		return
	}
	var body struct {
		ReleaseID string `json:"release_id"`
		Rebuild   bool   `json:"rebuild"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "project."+action, name, func() (any, error) {
		if action == "stop" {
			return map[string]bool{"stopped": true}, h.operations.StopProject(name)
		}
		return h.operations.Deploy(name, body.ReleaseID, action == "rollback", body.Rebuild)
	})
}
