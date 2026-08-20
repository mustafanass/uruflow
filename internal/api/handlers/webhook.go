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

package handlers

import (
	"io"
	"net/http"

	"github.com/mustafanass/uruflow/internal/services"
	"github.com/mustafanass/uruflow/pkg/helper"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const maxPayloadSize = 5 << 20

type WebhookHandler struct {
	service *services.WebhookService
}

func NewWebhookHandler(service *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{service: service}
}

func Health(w http.ResponseWriter, r *http.Request) {
	helper.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxPayloadSize+1))
	if err != nil {
		helper.WriteError(w, http.StatusBadRequest, "cannot read request body")
		return
	}
	if len(payload) > maxPayloadSize {
		helper.WriteError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}

	provider, ok := h.authenticate(r, payload)
	if !ok {
		logger.Warn("[WEBHOOK] rejected a request from %s", r.RemoteAddr)
		helper.WriteError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	deliveryID, ok := webhookHeaders(r, provider)
	if !ok {
		helper.WriteError(w, http.StatusBadRequest, "invalid webhook headers")
		return
	}
	claimed, err := h.service.ClaimDelivery(provider, deliveryID)
	if err != nil {
		logger.Error("[WEBHOOK] claim delivery: %v", err)
		helper.WriteError(w, http.StatusInternalServerError, "cannot record webhook delivery")
		return
	}
	if !claimed {
		helper.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "duplicate"})
		return
	}
	result, err := h.service.Handle(provider, payload)
	if err != nil {
		logger.Warn("[WEBHOOK] %v", err)
		helper.WriteError(w, http.StatusAccepted, err.Error())
		return
	}

	triggered := make([]map[string]string, 0, len(result.Outcomes))
	for _, outcome := range result.Outcomes {
		entry := map[string]string{"project": outcome.Project, "release": outcome.Release}
		if outcome.Err != nil {
			entry["error"] = outcome.Err.Error()
		}
		triggered = append(triggered, entry)
	}

	helper.WriteJSON(w, http.StatusAccepted, map[string]any{
		"repository": result.Repository,
		"branch":     result.Branch,
		"triggered":  triggered,
	})
}

func webhookHeaders(r *http.Request, provider string) (string, bool) {
	var deliveryID, event string
	switch provider {
	case services.ProviderGitHub:
		deliveryID = r.Header.Get("X-GitHub-Delivery")
		event = r.Header.Get("X-GitHub-Event")
		if event != "push" {
			return "", false
		}
	case services.ProviderGitLab:
		deliveryID = r.Header.Get("X-Gitlab-Event-UUID")
		event = r.Header.Get("X-Gitlab-Event")
		if event != "Push Hook" {
			return "", false
		}
	default:
		return "", false
	}
	return deliveryID, deliveryID != "" && len(deliveryID) <= 255
}

func (h *WebhookHandler) authenticate(r *http.Request, payload []byte) (string, bool) {
	if signature := r.Header.Get("X-Hub-Signature-256"); signature != "" {
		return services.ProviderGitHub, h.service.VerifyGitHub(payload, signature)
	}
	if token := r.Header.Get("X-Gitlab-Token"); token != "" {
		return services.ProviderGitLab, h.service.VerifyGitLab(token)
	}
	if r.Header.Get("X-GitHub-Event") != "" {
		return services.ProviderGitHub, h.service.VerifyGitHub(payload, "")
	}
	if r.Header.Get("X-Gitlab-Event") != "" {
		return services.ProviderGitLab, h.service.VerifyGitLab("")
	}
	return "", false
}
