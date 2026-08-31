//go:build live

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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/pki"
)

const (
	liveGate = "URUFLOW_DOCKER_TESTS"
	livePort = 5555
)

func liveRegistry(t *testing.T) *Registry {
	t.Helper()

	if os.Getenv(liveGate) == "" {
		t.Skip("set " + liveGate + "=1 to run tests that talk to a real docker daemon")
	}

	client, err := docker.New(docker.DefaultSocket)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "storage"), 0o755)

	authority, err := pki.LoadOrCreateCA(filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"))
	if err != nil {
		t.Fatalf("ca: %v", err)
	}

	certPath := filepath.Join(dir, "registry.crt")
	keyPath := filepath.Join(dir, "registry.key")
	if err := authority.EnsureLeaf(pki.Material{
		CertPath: certPath, KeyPath: keyPath,
		Names: []string{"127.0.0.1", "localhost"},
	}); err != nil {
		t.Fatalf("registry certificate: %v", err)
	}
	caPEM, _ := authority.CertificatePEM()

	options := Options{
		Address:      "127.0.0.1:5555",
		Hostname:     "127.0.0.1",
		Port:         livePort,
		Namespace:    "uruflow",
		Username:     "uruflow",
		Password:     "live-test-password",
		Image:        "registry:2",
		DataDir:      filepath.Join(dir, "storage"),
		CertPath:     certPath,
		KeyPath:      keyPath,
		HtpasswdPath: filepath.Join(dir, "htpasswd"),
		CACert:       caPEM,
	}

	instance := New(options, client)
	instance.containerName = fmt.Sprintf("uruflow-registry-live-%d", time.Now().UnixNano())
	t.Cleanup(func() { client.Remove(context.Background(), instance.containerName, true) })

	return instance
}

func TestLiveRegistryComesUpWithTLSAndAuth(t *testing.T) {
	registry := liveRegistry(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	started := time.Now()
	if err := registry.Ensure(ctx, func(status string) { t.Logf("  %s", status) }); err != nil {
		t.Fatalf("bring the registry up: %v", err)
	}
	t.Logf("registry healthy after %s", time.Since(started).Round(time.Millisecond))

	if err := registry.Health(ctx); err != nil {
		t.Fatalf("health: %v", err)
	}

	repositories, err := registry.Repositories(ctx)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	t.Logf("catalog returned %d repositories", len(repositories))

	if err := registry.Ensure(ctx, nil); err != nil {
		t.Fatalf("a second Ensure on a running registry should be a no-op: %v", err)
	}
}

func TestLiveRegistryRejectsAnonymousAndUntrustedCallers(t *testing.T) {
	registry := liveRegistry(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := registry.Ensure(ctx, nil); err != nil {
		t.Fatalf("bring the registry up: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(registry.Options().CACert))
	trusting := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}

	response, err := trusting.Get("https://" + registry.Options().Address + "/v2/")
	if err != nil {
		t.Fatalf("anonymous request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous request got %s, want 401", response.Status)
	}

	untrusting := &http.Client{Timeout: 10 * time.Second}
	if _, err := untrusting.Get("https://" + registry.Options().Address + "/v2/"); err == nil {
		t.Error("a client that does not trust the uruflow CA was able to connect")
	}

	plain, err := trusting.Get("http://" + registry.Options().Address + "/v2/")
	if err != nil {
		return
	}
	defer plain.Body.Close()

	if plain.StatusCode < http.StatusBadRequest {
		t.Errorf("a plaintext request was served with %s, want a rejection", plain.Status)
	}
}
