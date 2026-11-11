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

package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"golang.org/x/crypto/bcrypt"
)

const (
	ContainerName  = "uruflow-registry"
	Realm          = "uruflow"
	internalPort   = 5000
	certTarget     = "/certs/registry.crt"
	keyTarget      = "/certs/registry.key"
	htpasswdTarget = "/auth/htpasswd"
	storageTarget  = "/var/lib/registry"
	startTimeout   = 60 * time.Second
	stopTimeout    = 10 * time.Second
)

type Options struct {
	Address      string
	Hostname     string
	Port         int
	Namespace    string
	Username     string
	Password     string
	Image        string
	DataDir      string
	CertPath     string
	KeyPath      string
	HtpasswdPath string
	CACert       string
}

type Registry struct {
	options       Options
	docker        *docker.Client
	containerName string
	clientOnce    sync.Once
	client        *http.Client
	clientErr     error
	catalogCache  catalogCache
	maintaining   atomic.Bool
	credentialsMu sync.RWMutex
}

func New(options Options, engine *docker.Client) *Registry {
	r := &Registry{options: options, docker: engine, containerName: ContainerName}
	r.catalogCache.wake = make(chan struct{}, 1)
	r.loadCatalog()
	return r
}

func (r *Registry) Options() Options { return r.options }

func (r *Registry) Ensure(ctx context.Context, onProgress func(string)) error {
	if err := r.docker.Remove(ctx, r.containerName+"-gc", true); err != nil && !errors.Is(err, docker.ErrNotFound) {
		return fmt.Errorf("recover registry maintenance: %w", err)
	}
	if err := r.writeHtpasswd(); err != nil {
		return fmt.Errorf("registry credentials: %w", err)
	}

	state, err := r.state(ctx)
	if err != nil {
		return err
	}

	switch state {
	case stateRunning:
		return r.waitHealthy(ctx)
	case stateStopped:
		if err := r.docker.Start(ctx, r.containerName); err != nil {
			return fmt.Errorf("start registry: %w", err)
		}
		return r.waitHealthy(ctx)
	}

	if !r.docker.HasImage(ctx, r.options.Image) {
		if err := r.docker.Pull(ctx, r.options.Image, nil, onProgress); err != nil {
			return fmt.Errorf("pull %s: %w", r.options.Image, err)
		}
	}

	if _, err := r.docker.Run(ctx, r.spec()); err != nil {
		return fmt.Errorf("run registry: %w", err)
	}
	return r.waitHealthy(ctx)
}

func (r *Registry) Stop(ctx context.Context) error {
	return r.docker.Stop(ctx, r.containerName, stopTimeout)
}

func (r *Registry) spec() docker.Spec {
	return docker.Spec{
		Name:  r.containerName,
		Image: r.options.Image,
		Env: map[string]string{
			"REGISTRY_HTTP_ADDR":              fmt.Sprintf("0.0.0.0:%d", internalPort),
			"REGISTRY_HTTP_TLS_CERTIFICATE":   certTarget,
			"REGISTRY_HTTP_TLS_KEY":           keyTarget,
			"REGISTRY_AUTH":                   "htpasswd",
			"REGISTRY_AUTH_HTPASSWD_REALM":    Realm,
			"REGISTRY_AUTH_HTPASSWD_PATH":     htpasswdTarget,
			"REGISTRY_STORAGE_DELETE_ENABLED": "true",
		},
		Ports: []docker.PortBinding{{Host: r.options.Port, Container: internalPort}},
		Mounts: []docker.Mount{
			{Source: r.options.CertPath, Target: certTarget, ReadOnly: true},
			{Source: r.options.KeyPath, Target: keyTarget, ReadOnly: true},
			{Source: r.options.HtpasswdPath, Target: htpasswdTarget, ReadOnly: true},
			{Source: r.options.DataDir, Target: storageTarget},
		},
		Labels: map[string]string{
			docker.LabelManaged: "true",
			docker.LabelRole:    docker.RoleRegistry,
		},
		Restart: "unless-stopped",
	}
}

type containerState int

const (
	stateMissing containerState = iota
	stateStopped
	stateRunning
)

func (r *Registry) state(ctx context.Context) (containerState, error) {
	containers, err := r.docker.ListContainers(ctx, false)
	if err != nil {
		return stateMissing, err
	}

	for _, container := range containers {
		if container.Name != r.containerName {
			continue
		}
		if container.State == docker.StateRunning {
			return stateRunning, nil
		}
		return stateStopped, nil
	}
	return stateMissing, nil
}

func (r *Registry) waitHealthy(ctx context.Context) error {
	deadline := time.Now().Add(startTimeout)
	var lastErr error

	for time.Now().Before(deadline) {
		if lastErr = r.Health(ctx); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("registry did not become healthy on %s: %w", r.options.Address, lastErr)
}

func (r *Registry) SetPassword(ctx context.Context, password string) error {
	if !r.maintaining.CompareAndSwap(false, true) {
		return fmt.Errorf("registry maintenance is already running")
	}
	defer r.maintaining.Store(false)
	r.credentialsMu.Lock()
	r.options.Password = password
	r.credentialsMu.Unlock()
	if err := r.writeHtpasswd(); err != nil {
		return fmt.Errorf("registry credentials: %w", err)
	}
	if err := r.Stop(ctx); err != nil {
		return err
	}
	if err := r.docker.Start(ctx, r.containerName); err != nil {
		return fmt.Errorf("start registry: %w", err)
	}
	return r.waitHealthy(ctx)
}

func (r *Registry) credentials() (string, string) {
	r.credentialsMu.RLock()
	defer r.credentialsMu.RUnlock()
	return r.options.Username, r.options.Password
}

func (r *Registry) writeHtpasswd() error {
	username, password := r.credentials()
	if current, err := os.ReadFile(r.options.HtpasswdPath); err == nil {
		if credentialsMatch(current, username, password) {
			return nil
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	line := fmt.Sprintf("%s:%s\n", username, hash)
	return os.WriteFile(r.options.HtpasswdPath, []byte(line), 0o600)
}

func credentialsMatch(current []byte, username, password string) bool {
	user, hash, found := strings.Cut(strings.TrimSpace(string(current)), ":")
	if !found || user != username {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (r *Registry) Maintaining() bool { return r.maintaining.Load() }
