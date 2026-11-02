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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/observe"
	"github.com/mustafanass/uruflow/internal/ops"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func prepareStream(w http.ResponseWriter) func(string, string, any) error {
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	return func(event, id string, value any) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if id != "" {
			if _, err = fmt.Fprintf(w, "id: %s\n", id); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
			return err
		}
		return controller.Flush()
	}
}
func (h *Handler) stream(w http.ResponseWriter, r *http.Request, user *storage.ConsoleUser) {
	release, ok := h.reserveStream(w, user.Username)
	if !ok {
		return
	}
	defer release()
	channel := make(chan observe.Snapshot, 1)
	h.mu.Lock()
	h.subscribers[channel] = struct{}{}
	if h.latest != nil {
		channel <- *h.latest
	}
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.subscribers, channel); h.mu.Unlock() }()
	send := prepareStream(w)
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	var previous *observe.Snapshot
	for {
		select {
		case <-r.Context().Done():
			return
		case <-h.ctx.Done():
			return
		case snapshot := <-channel:
			_, current, err := h.auth.identify(r)
			if err != nil {
				_ = send("session-expired", "", nil)
				return
			}
			visible := visibleSnapshot(snapshot, current)
			if previous == nil {
				if err := send("snapshot", "", visible); err != nil {
					return
				}
			} else if delta := snapshotChanges(*previous, visible); len(delta) > 0 {
				if err := send("snapshot-update", "", delta); err != nil {
					return
				}
			}
			previous = &visible
		case <-heartbeat.C:
			if _, _, err := h.auth.identify(r); err != nil {
				_ = send("session-expired", "", nil)
				return
			}
			if send("heartbeat", "", map[string]any{"at": time.Now()}) != nil {
				return
			}
		}
	}
}

func snapshotChanges(previous, current observe.Snapshot) map[string]json.RawMessage {
	before, _ := json.Marshal(previous)
	after, _ := json.Marshal(current)
	old, next := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	_ = json.Unmarshal(before, &old)
	_ = json.Unmarshal(after, &next)
	changed := map[string]json.RawMessage{}
	for key, value := range next {
		if key != "at" && string(old[key]) != string(value) {
			changed[key] = value
		}
	}
	if len(changed) > 0 {
		changed["at"] = next["at"]
	}
	return changed
}

type logEvent struct {
	kind, id string
	value    any
}

func (h *Handler) logStream(w http.ResponseWriter, r *http.Request, user *storage.ConsoleUser, produce func(context.Context, func(logEvent) error) error) {
	release, ok := h.reserveStream(w, user.Username)
	if !ok {
		return
	}
	defer release()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	entries := make(chan logEvent, 64)
	go func() {
		defer close(entries)
		err := produce(ctx, func(event logEvent) error {
			select {
			case entries <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil && ctx.Err() == nil {
			retryable := true
			var failure *ops.LogStreamError
			if errors.As(err, &failure) && failure.Code >= 400 && failure.Code < 500 {
				retryable = false
			}
			if errors.Is(err, storage.ErrNotFound) {
				retryable = false
			}
			select {
			case entries <- logEvent{kind: "failure", value: map[string]any{"error": err.Error(), "retryable": retryable}}:
			case <-ctx.Done():
			}
		}
	}()
	send := prepareStream(w)
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.ctx.Done():
			return
		case <-heartbeat.C:
			if _, _, err := h.auth.identify(r); err != nil {
				_ = send("session-expired", "", nil)
				return
			}
			if send("heartbeat", "", nil) != nil {
				return
			}
		case event, ok := <-entries:
			if !ok {
				_ = send("complete", "", nil)
				return
			}
			if _, _, err := h.auth.identify(r); err != nil {
				_ = send("session-expired", "", nil)
				return
			}
			if send(event.kind, event.id, event.value) != nil {
				return
			}
			if event.kind == "failure" {
				return
			}
		}
	}
}
func (h *Handler) releaseLogs(c *gin.Context) {
	w, r, u := c.Writer, c.Request, currentUser(c)
	id := c.Param("id")
	release, err := h.operations.Release(id)
	if err != nil || !u.CanProject(release.Project) {
		writeError(w, 404, "Release not found.")
		return
	}
	after, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if value, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64); err == nil && value > after {
		after = value
	}
	if after < 0 {
		after = 0
	}
	h.logStream(w, r, u, func(ctx context.Context, emit func(logEvent) error) error {
		return h.operations.FollowReleaseLogs(ctx, id, after, func(line models.LogLine) error { return emit(logEvent{"log", strconv.FormatInt(line.ID, 10), line}) })
	})
}
func (h *Handler) containerLogs(c *gin.Context) {
	w, r, u := c.Writer, c.Request, currentUser(c)
	after := r.URL.Query().Get("after")
	if after == "" {
		after = r.Header.Get("Last-Event-ID")
	}
	if _, err := ufp.ParseLogPosition(after); err != nil {
		writeError(w, 400, "Invalid log cursor.")
		return
	}
	container, err := h.operations.Container(c.Param("agent"), c.Param("id"))
	if err != nil || (!u.AllProjects() && (container.Project == "" || !u.CanProject(container.Project))) {
		writeError(w, 404, "Container not found.")
		return
	}
	h.logStream(w, r, u, func(ctx context.Context, emit func(logEvent) error) error {
		return h.operations.FollowContainerLogs(ctx, c.Param("agent"), c.Param("id"), after, func(line ufp.ContainerLog) error { return emit(logEvent{kind: "log", id: line.Cursor, value: line}) })
	})
}

func (h *Handler) reserveStream(w http.ResponseWriter, username string) (func(), bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	total := 0
	for _, n := range h.streams {
		total += n
	}
	if h.streams[username] >= 12 || total >= 256 {
		writeError(w, 429, "Too many live streams. Close another view and retry.")
		return nil, false
	}
	h.streams[username]++
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.streams[username]--
		if h.streams[username] == 0 {
			delete(h.streams, username)
		}
	}, true
}
