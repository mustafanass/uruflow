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
	"crypto/tls"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mustafanass/uruflow/internal/api/middleware"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/observe"
	"github.com/mustafanass/uruflow/internal/ops"
	"github.com/mustafanass/uruflow/internal/pipeline"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/pkg/logger"
)

//go:embed all:assets
var embedded embed.FS

type Handler struct {
	http        *http.Server
	operations  Operator
	accounts    storage.ConsoleStore
	auth        *authentication
	router      http.Handler
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.Mutex
	latest      *observe.Snapshot
	subscribers map[chan observe.Snapshot]struct{}
	streams     map[string]int
}

func New(operations Operator, accounts storage.ConsoleStore, dataDir string, secureCookies bool) (*Handler, error) {
	auth, err := newAuthentication(accounts, dataDir, secureCookies)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &Handler{operations: operations, accounts: accounts, auth: auth, ctx: ctx, cancel: cancel, subscribers: map[chan observe.Snapshot]struct{}{}, streams: map[string]int{}}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.RedirectTrailingSlash = false
	router.UseRawPath = true
	if err := router.SetTrustedProxies(nil); err != nil {
		cancel()
		return nil, err
	}
	router.Use(middleware.Logging(), middleware.Recovery())
	h.routes(router)
	h.router = http.NewCrossOriginProtection().Handler(router)
	return h, nil
}
func (h *Handler) Start() { h.wg.Add(1); go h.observe() }
func (h *Handler) Close() {
	h.cancel()
	if h.http != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = h.http.Shutdown(ctx)
	}
	h.wg.Wait()
}
func (h *Handler) SetupTokenPath() string { return h.auth.bootstrapPath }
func AssetsReady() bool                   { _, err := fs.Stat(embedded, "assets/index.html"); return err == nil }
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
	if h.auth.secureCookies || r.TLS != nil {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	}
	h.router.ServeHTTP(w, r)
}
func (h *Handler) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, 405, "Method not allowed.")
		return
	}
	files, _ := fs.Sub(embedded, "assets")
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if _, err := fs.Stat(files, name); err != nil {
		if strings.Contains(path.Base(name), ".") {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
	}
	serveAsset(w, r, files, name)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func readJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeError(w, 400, "Invalid request body.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, 400, "Request must contain one JSON object.")
		return false
	}
	return true
}
func rank(role string) int {
	switch role {
	case "admin":
		return 3
	case "operator":
		return 2
	case "viewer":
		return 1
	}
	return 0
}
func (h *Handler) record(actor, action, resource, outcome string) error {
	return h.accounts.AppendAudit(&storage.AuditEntry{Actor: actor, Action: action, Resource: resource, Outcome: outcome})
}
func (h *Handler) mutation(w http.ResponseWriter, r *http.Request, user *storage.ConsoleUser, action, resource string, run func() (any, error)) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(3 * time.Minute))
	if err := h.record(user.Username, action, resource, "requested"); err != nil {
		writeError(w, 503, "Audit storage is unavailable; the action was not started.")
		return
	}
	value, err := run()
	outcome := "succeeded"
	if err != nil {
		outcome = "failed"
	}
	auditErr := h.record(user.Username, action, resource, outcome)
	if err != nil {
		status := 400
		if errors.Is(err, errForbidden) {
			status = 403
		}
		if errors.Is(err, storage.ErrNotFound) {
			status = 404
		}
		if errors.Is(err, pipeline.ErrRegistryMaintenance) || errors.Is(err, pipeline.ErrRegistryUnavailable) {
			status = 503
			w.Header().Set("Retry-After", "30")
		}
		if errors.Is(err, ops.ErrConflict) {
			status = 409
		}
		writeError(w, status, err.Error())
		return
	}
	if auditErr != nil {
		w.Header().Set("X-Uruflow-Audit-Warning", "outcome-record-failed")
	}
	writeJSON(w, 200, value)
}
func (h *Handler) observe() {
	defer h.wg.Done()
	ticker := time.NewTicker(ops.ObservationInterval)
	defer ticker.Stop()
	for {
		snapshot, err := h.operations.Snapshot(h.ctx)
		if err == nil {
			h.mu.Lock()
			h.latest = &snapshot
			for subscriber := range h.subscribers {
				select {
				case subscriber <- snapshot:
				default:
					select {
					case <-subscriber:
					default:
					}
					select {
					case subscriber <- snapshot:
					default:
					}
				}
			}
			h.mu.Unlock()
		}
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) Listen(cfg *config.Config) error {
	listener, err := net.Listen("tcp", cfg.WebAddr())
	if err != nil {
		return err
	}
	if cfg.Web.TLS {
		certPath, keyPath := cfg.WebCertificatePaths()
		pair, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			listener.Close()
			return err
		}
		listener = tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS13, NextProtos: []string{"h2", "http/1.1"}, Certificates: []tls.Certificate{pair}})
	}
	var handler http.Handler = h
	if cfg.Web.PublicURL != "" {
		origin, _ := url.Parse(cfg.Web.PublicURL)
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.EqualFold(r.Host, origin.Host) {
				writeError(w, 421, "Use the configured workspace address.")
				return
			}
			h.ServeHTTP(w, r)
		})
	}
	h.http = &http.Server{Handler: handler, ReadTimeout: 10 * time.Second, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 16 << 10, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	h.http.Protocols = new(http.Protocols)
	h.http.Addr = listener.Addr().String()
	h.http.Protocols.SetHTTP1(true)
	h.http.Protocols.SetHTTP2(true)
	h.Start()
	go func() {
		if err := h.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("[WEB] listener failed: %v", err)
		}
	}()
	return nil
}
