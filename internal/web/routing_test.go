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
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mustafanass/uruflow/internal/models"
)

func TestGinRoutingPreservesAPIAndBrowserBoundaries(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	for _, test := range []struct {
		method, path string
		status       int
		content      string
	}{
		{"GET", "/api/missing", 404, "application/json"},
		{"GET", "/api", 404, "application/json"},
		{"PATCH", "/api/session", 405, "application/json"},
		{"POST", "/api/session/", 404, "application/json"},
		{"GET", "/api//session", 404, "application/json"},
		{"HEAD", "/api/session", 200, "application/json"},
		{"GET", "/projects/demo", 200, "text/html"},
		{"HEAD", "/projects/demo", 200, "text/html"},
		{"POST", "/projects/demo", 405, "application/json"},
	} {
		w := request(h, test.method, test.path, "", nil, "")
		if w.Code != test.status || !strings.Contains(w.Header().Get("Content-Type"), test.content) {
			t.Fatalf("%s %s: %d %s", test.method, test.path, w.Code, w.Body.String())
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("security headers missing")
		}
	}
}

func TestGinKeepsStrictBodiesAndDecodesScopedRouteParameters(t *testing.T) {
	operator := &fixtureOperator{}
	h, _ := newConsole(t, operator, true)
	cookie, csrf := signIn(t, h, "admin")
	for _, body := range []string{`{"unknown":true}`, `{} {}`, `{"release_id":true}`, strings.Repeat("x", (1<<20)+1)} {
		w := request(h, "POST", "/api/projects/assigned-project/stop", body, cookie, csrf)
		if w.Code != http.StatusBadRequest || operator.calls != 0 {
			t.Fatalf("invalid body reached mutation: %d %d", w.Code, operator.calls)
		}
	}
	viewer, _ := signIn(t, h, "viewer")
	for _, name := range []string{"assigned-project", "assigned%2Dproject"} {
		w := request(h, "GET", "/api/projects/"+name+"/configuration", "", viewer, "")
		if w.Code != 403 {
			t.Fatalf("encoded route bypassed role check: %d", w.Code)
		}
	}
}

type panickingOperator struct{ fixtureOperator }

func (*panickingOperator) Images(context.Context) ([]models.Image, error) {
	panic("test handler panic")
}

func TestGinRecoveryContainsHandlerPanics(t *testing.T) {
	h, _ := newConsole(t, &panickingOperator{}, true)
	cookie, _ := signIn(t, h, "admin")
	w := request(h, "GET", "/api/images", "", cookie, "")
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"error":"Internal Server Error"`) || strings.Contains(w.Body.String(), "test handler panic") {
		t.Fatal(w.Code, w.Body.String())
	}
	if next := request(h, "GET", "/api/session", "", cookie, ""); next.Code != 200 {
		t.Fatal("listener failed after a panic", next.Code)
	}
}
