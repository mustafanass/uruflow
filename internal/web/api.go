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

func (h *Handler) routes(router *gin.Engine) {
	api := router.Group("/api")
	api.Match([]string{"GET", "HEAD"}, "/session", h.auth.sessionStatus)
	api.POST("/session", h.auth.login)
	api.POST("/setup", h.auth.setup)

	authenticated := api.Group("", h.authenticate())
	authenticated.DELETE("/session", h.auth.logout)
	authenticated.POST("/session/refresh", h.auth.refresh)
	authenticated.Match([]string{"GET", "HEAD"}, "/snapshot", h.snapshot)
	authenticated.GET("/stream", h.liveStream)
	authenticated.Match([]string{"GET", "HEAD"}, "/trust-root", h.requireRole("operator"), h.trustRoot)

	agents := authenticated.Group("/agents", h.requireRole("admin"))
	agents.POST("", h.enrollAgent)
	agents.DELETE("/:name", h.removeAgent)

	projects := authenticated.Group("/projects")
	projects.POST("", h.requireRole("operator"), h.createProject)
	projects.POST("/reload", h.requireRole("operator"), h.reloadProjects)
	projects.Match([]string{"GET", "HEAD"}, "/:name/releases", h.projectReleases)
	documents := projects.Group("/:name", h.requireRole("operator"))
	documents.Match([]string{"GET", "HEAD"}, "/configuration", h.getProjectDocument)
	documents.PUT("/configuration", h.saveProjectDocument)
	documents.Match([]string{"GET", "HEAD"}, "/variables", h.getProjectDocument)
	documents.PUT("/variables", h.saveProjectDocument)
	documents.POST("/deploy", h.projectAction)
	documents.POST("/rollback", h.projectAction)
	documents.POST("/stop", h.projectAction)

	releases := authenticated.Group("/releases")
	releases.Match([]string{"GET", "HEAD"}, "/:id", h.release)
	releases.GET("/:id/logs", h.releaseLogs)
	authenticated.GET("/containers/:agent/:id/logs", h.containerLogs)
	containers := authenticated.Group("/containers/:agent/:id", h.requireRole("operator"))
	containers.POST("/restart", h.restartContainer)
	containers.GET("/terminal", h.containerTerminal)
	containers.POST("/terminal/:session/input", h.terminalInput)
	containers.POST("/terminal/:session/resize", h.resizeTerminal)
	authenticated.Match([]string{"GET", "HEAD"}, "/images", h.images)
	authenticated.Match([]string{"GET", "HEAD"}, "/registry/catalog", h.images)
	authenticated.Match([]string{"GET", "HEAD"}, "/images/details", h.imageDetails)
	authenticated.DELETE("/images", h.requireRole("admin"), h.removeImage)
	authenticated.POST("/alerts/:id/resolve", h.requireRole("operator"), h.resolveAlert)
	authenticated.Match([]string{"GET", "HEAD"}, "/alerts", h.alerts)

	users := authenticated.Group("/users", h.requireRole("admin"))
	users.Match([]string{"GET", "HEAD"}, "", h.users)
	users.POST("", h.createUser)
	users.DELETE("/:name", h.removeUser)
	users.PUT("/:name/role", h.changeRole)
	users.PUT("/:name/projects", h.changeProjects)
	users.PUT("/:name/password", h.resetPassword)

	account := authenticated.Group("/account")
	account.DELETE("/sessions", h.revokeSessions)
	account.POST("/password", h.changePassword)
	authenticated.Match([]string{"GET", "HEAD"}, "/audit", h.requireRole("admin"), h.audit)

	router.NoMethod(func(c *gin.Context) { writeError(c.Writer, 405, "Method not allowed.") })
	router.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/api" || strings.HasPrefix(c.Request.URL.Path, "/api/") {
			writeError(c.Writer, 404, "API route not found.")
			return
		}
		c.Status(200)
		h.static(c.Writer, c.Request)
	})
}

func (h *Handler) liveStream(c *gin.Context) {
	h.stream(c.Writer, c.Request, currentUser(c))
}
