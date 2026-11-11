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
	"testing"

	"github.com/mustafanass/uruflow/internal/ops"
	"github.com/mustafanass/uruflow/internal/pipeline"
)

var settingsRoutes = []string{"GET /api/settings/webhook", "PUT /api/settings/webhook", "POST /api/settings/webhook/secret", "PUT /api/settings/webhook/certificate", "GET /api/settings/webhook/deliveries", "GET /api/settings/registry", "PUT /api/settings/registry", "POST /api/settings/registry/password", "GET /api/settings/agents", "PUT /api/settings/agents"}

type settingsOperator struct {
	failure error
	fixtureOperator
}

func (f *settingsOperator) SaveWebhookSettings(change ops.WebhookChange) (ops.WebhookView, error) {
	f.calls++
	return ops.WebhookView{Revision: change.Revision + 1, Enabled: change.Enabled}, f.failure
}

func (f *settingsOperator) RotateRegistryPassword(revision int64) (ops.RegistryView, error) {
	f.calls++
	return ops.RegistryView{Revision: revision + 1}, f.failure
}

func TestSettingsRequireAnAdministrator(t *testing.T) {
	operator := &settingsOperator{}
	handler, _ := newConsole(t, operator, true)
	for _, route := range settingsRoutes {
		method, path, _ := strings.Cut(route, " ")
		if response := request(handler, method, path, `{}`, nil, ""); response.Code != 401 {
			t.Fatalf("anonymous %s: %d", route, response.Code)
		}
	}
	for _, user := range []string{"operator", "viewer"} {
		cookie, csrf := signIn(t, handler, user)
		for _, route := range settingsRoutes {
			method, path, _ := strings.Cut(route, " ")
			if response := request(handler, method, path, `{}`, cookie, csrf); response.Code != 403 {
				t.Fatalf("%s %s: %d %s", user, route, response.Code, response.Body)
			}
		}
	}
	if operator.calls != 0 {
		t.Fatal("a non-administrator reached the settings backend")
	}
}

func TestSettingsChangesAreAuditedAndReportConflicts(t *testing.T) {
	operator := &settingsOperator{}
	handler, store := newConsole(t, operator, true)
	admin, csrf := signIn(t, handler, "admin")
	if response := request(handler, "PUT", "/api/settings/webhook", `{"revision":1}`, admin, ""); response.Code != 403 || operator.calls != 0 {
		t.Fatalf("settings changed without CSRF: %d", response.Code)
	}
	if response := request(handler, "PUT", "/api/settings/webhook", `{"revision":1}`, admin, csrf); response.Code != 200 || operator.calls != 1 {
		t.Fatalf("save: %d %s", response.Code, response.Body)
	}
	entries, err := store.ListAudit(10)
	if err != nil || len(entries) < 2 || entries[0].Action != "settings.webhook.update" || entries[0].Outcome != "succeeded" || entries[1].Outcome != "requested" || entries[0].Actor != "admin" {
		t.Fatalf("settings change not audited: %+v %v", entries, err)
	}
	for _, failure := range []error{ops.ErrSettingsConflict, pipeline.ErrRegistryBusy} {
		operator.failure = failure
		response := request(handler, "POST", "/api/settings/registry/password", `{"revision":1}`, admin, csrf)
		if response.Code != 409 || !strings.Contains(response.Body.String(), failure.Error()) {
			t.Fatalf("%v: %d %s", failure, response.Code, response.Body)
		}
	}
}
