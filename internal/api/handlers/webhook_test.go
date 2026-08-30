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
	"net/http/httptest"
	"testing"

	"github.com/mustafanass/uruflow/internal/services"
)

func TestWebhookHeadersRequirePushAndDeliveryID(t *testing.T) {
	request := httptest.NewRequest("POST", "/webhook", nil)
	request.Header.Set("X-GitHub-Event", "push")
	request.Header.Set("X-GitHub-Delivery", "delivery-1")
	if id, ok := webhookHeaders(request, services.ProviderGitHub); !ok || id != "delivery-1" {
		t.Fatalf("valid headers = %q, %v", id, ok)
	}
	request.Header.Set("X-GitHub-Event", "issues")
	if _, ok := webhookHeaders(request, services.ProviderGitHub); ok {
		t.Fatal("non-push event was accepted")
	}
	request.Header.Set("X-GitHub-Event", "push")
	request.Header.Del("X-GitHub-Delivery")
	if _, ok := webhookHeaders(request, services.ProviderGitHub); ok {
		t.Fatal("event without a delivery id was accepted")
	}
}
