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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/observe"
	"github.com/mustafanass/uruflow/internal/ops"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/ufp"
	"golang.org/x/crypto/bcrypt"
)

func TestSessionSurvivesStoreAndHandlerRestart(t *testing.T) {
	h, store := newConsole(t, &fixtureOperator{}, true)
	cookie, csrf := signIn(t, h, "admin")
	path := filepath.Join(filepath.Dir(h.SetupTokenPath()), "console.db")
	h.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	next, err := New(&fixtureOperator{}, reopened, t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	w := request(next, "GET", "/api/session", "", cookie, "")
	var value struct {
		Authenticated bool   `json:"authenticated"`
		CSRF          string `json:"csrf_token"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if !value.Authenticated || value.CSRF != csrf {
		t.Fatal("valid session was lost during restart")
	}
	if w = request(next, "DELETE", "/api/session", "", cookie, csrf); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	third, err := New(&fixtureOperator{}, reopened, t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	r := httptest.NewRequest("GET", "https://workspace.test/api/session", nil)
	r.AddCookie(cookie)
	if _, _, err = third.auth.identify(r); err == nil {
		t.Fatal("signed-out session survived restart")
	}
}

func TestLocalPasswordRecoveryRevokesActiveAndPersistedSessions(t *testing.T) {
	h, store := newConsole(t, &fixtureOperator{}, true)
	cookie, _ := signIn(t, h, "admin")
	hash, err := bcrypt.GenerateFromPassword([]byte("replacement administrator password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ResetConsoleAdministrator("admin", hash); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "https://workspace.test/api/session", nil)
	r.AddCookie(cookie)
	if _, _, err = h.auth.identify(r); err == nil {
		t.Fatal("recovered administrator retained an active session")
	}
	sessions, err := store.ListConsoleSessions()
	if err != nil || len(sessions) != 0 {
		t.Fatal("recovered administrator retained persisted sessions", sessions, err)
	}
}

func TestSnapshotChangesSkipClockOnlyUpdates(t *testing.T) {
	previous := observe.Snapshot{At: time.Now(), Registry: "healthy"}
	next := previous
	next.At = next.At.Add(time.Second)
	if len(snapshotChanges(previous, next)) != 0 {
		t.Fatal("clock-only snapshot was sent")
	}
	next.Registry = "unavailable"
	changes := snapshotChanges(previous, next)
	if len(changes) != 2 || string(changes["registry"]) != `"unavailable"` {
		t.Fatal(changes)
	}
}

type resumedLogOperator struct {
	fixtureOperator
	after string
}

func (f *resumedLogOperator) FollowContainerLogs(_ context.Context, _, _, after string, emit func(ufp.ContainerLog) error) error {
	f.after = after
	return emit(ufp.ContainerLog{Cursor: "2026-11-01T12:00:01Z/1", Line: "after reconnect", Timestamp: 1793534401})
}

func TestContainerLogsForwardResumeCursorAndReturnStableEventIDs(t *testing.T) {
	operator := &resumedLogOperator{}
	h, _ := newConsole(t, operator, true)
	cookie, _ := signIn(t, h, "admin")
	cursor := "2026-11-01T12:00:00Z/2"
	for _, header := range []bool{false, true} {
		address := "https://workspace.test/api/containers/agent/container/logs"
		if !header {
			address += "?after=" + url.QueryEscape(cursor)
		}
		r := httptest.NewRequest(http.MethodGet, address, nil)
		r.AddCookie(cookie)
		if header {
			r.Header.Set("Last-Event-ID", cursor)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || operator.after != cursor || !strings.Contains(w.Body.String(), "id: 2026-11-01T12:00:01Z/1\n") || !strings.Contains(w.Body.String(), "event: complete") {
			t.Fatal(w.Code, operator.after, w.Body.String())
		}
	}
}

type removedLogOperator struct{ fixtureOperator }

func (f *removedLogOperator) FollowContainerLogs(context.Context, string, string, string, func(ufp.ContainerLog) error) error {
	return &ops.LogStreamError{Code: 404, Message: "The container was removed."}
}

func TestRemovedContainerProducesATerminalStreamFailure(t *testing.T) {
	h, _ := newConsole(t, &removedLogOperator{}, true)
	cookie, _ := signIn(t, h, "admin")
	w := request(h, "GET", "/api/containers/agent/container/logs", "", cookie, "")
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "event: failure") || !strings.Contains(body, `"retryable":false`) || strings.Contains(body, "event: complete") {
		t.Fatal(w.Code, body)
	}
	h.mu.Lock()
	h.streams["admin"] = 12
	h.mu.Unlock()
	w = request(h, "GET", "/api/containers/agent/container/logs", "", cookie, "")
	if w.Code != 429 || strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal("stream limit did not return an HTTP error", w.Code, w.Body.String())
	}
}
