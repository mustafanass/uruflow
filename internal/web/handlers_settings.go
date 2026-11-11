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
	"github.com/mustafanass/uruflow/internal/ops"
)

func (h *Handler) webhookSettings(c *gin.Context) {
	w := c.Writer
	settings, err := h.operations.WebhookSettings()
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	writeJSON(w, 200, settings)
}

func (h *Handler) saveWebhookSettings(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body ops.WebhookChange
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "settings.webhook.update", "webhook", func() (any, error) {
		return h.operations.SaveWebhookSettings(body)
	})
}

func (h *Handler) rotateWebhookSecret(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct {
		Revision int64 `json:"revision"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "settings.webhook.secret.rotate", "webhook", func() (any, error) {
		return h.operations.RotateWebhookSecret(body.Revision)
	})
}

func (h *Handler) saveWebhookCertificate(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct {
		Revision    int64  `json:"revision"`
		Certificate string `json:"certificate"`
		Key         string `json:"key"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	action := "settings.webhook.certificate.update"
	if body.Certificate == "" && body.Key == "" {
		action = "settings.webhook.certificate.remove"
	}
	h.mutation(w, r, u, action, "webhook", func() (any, error) {
		return h.operations.SetWebhookCertificate(body.Revision, body.Certificate, body.Key)
	})
}

func (h *Handler) webhookDeliveries(c *gin.Context) {
	w := c.Writer
	deliveries, err := h.operations.WebhookDeliveries()
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	writeJSON(w, 200, deliveries)
}

func (h *Handler) registrySettings(c *gin.Context) {
	w := c.Writer
	settings, err := h.operations.RegistrySettings()
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	writeJSON(w, 200, settings)
}

func (h *Handler) saveRegistrySettings(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body ops.RegistryChange
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "settings.registry.update", "registry", func() (any, error) {
		return h.operations.SaveRegistrySettings(body)
	})
}

func (h *Handler) rotateRegistryPassword(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body struct {
		Revision int64 `json:"revision"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "settings.registry.password.rotate", "registry", func() (any, error) {
		return h.operations.RotateRegistryPassword(body.Revision)
	})
}

func (h *Handler) agentReporting(c *gin.Context) {
	w := c.Writer
	settings, err := h.operations.AgentReporting()
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	writeJSON(w, 200, settings)
}

func (h *Handler) saveAgentReporting(c *gin.Context) {
	w := c.Writer
	r := c.Request
	u := currentUser(c)
	var body ops.AgentReportingChange
	if !readJSON(w, r, &body) {
		return
	}
	h.mutation(w, r, u, "settings.agents.update", "agents", func() (any, error) {
		return h.operations.SaveAgentReporting(body)
	})
}
