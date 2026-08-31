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

package runner

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestHTTPReadinessAcceptsOnlyTwoHundreds(t *testing.T) {
	status := http.StatusNoContent
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/ready" {
			t.Errorf("path = %q", request.URL.Path)
		}
		response.WriteHeader(status)
	}))
	defer server.Close()
	engine := &fakeEngine{endpoint: server.Listener.Addr().String()}
	runner := New(engine, nil)
	check := &ufp.HealthcheckSpec{Type: "http", Scheme: "http", Path: "/ready", Port: 8080, Interval: time.Millisecond, Timeout: time.Second, Retries: 1}
	if err := runner.waitReady(context.Background(), "new", check); err != nil {
		t.Fatalf("2xx healthcheck failed: %v", err)
	}
	status = http.StatusServiceUnavailable
	if err := runner.waitReady(context.Background(), "new", check); err == nil {
		t.Fatal("503 healthcheck succeeded")
	}
}

func TestTCPReadinessSucceedsAndFailureIsBounded(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	engine := &fakeEngine{endpoint: listener.Addr().String()}
	runner := New(engine, nil)
	check := &ufp.HealthcheckSpec{Type: "tcp", Port: 8080, Interval: time.Millisecond, Timeout: 50 * time.Millisecond, Retries: 2}
	if err := runner.waitReady(context.Background(), "new", check); err != nil {
		t.Fatalf("tcp healthcheck failed: %v", err)
	}

	listener.Close()
	started := time.Now()
	if err := runner.waitReady(context.Background(), "new", check); err == nil {
		t.Fatal("closed tcp endpoint was healthy")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatalf("retry behavior was not bounded: %s", time.Since(started))
	}
}

func TestRunningReadinessRequiresStableWindow(t *testing.T) {
	engine := &fakeEngine{}
	runner := New(engine, nil)
	started := time.Now()
	if err := runner.waitReady(context.Background(), "new", &ufp.HealthcheckSpec{Type: "running", StableFor: 120 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 120*time.Millisecond {
		t.Fatalf("stable window ended early: %s", time.Since(started))
	}
}

func TestRunningReadinessHonorsCancellation(t *testing.T) {
	engine := &fakeEngine{}
	runner := New(engine, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := runner.waitReady(ctx, "new", &ufp.HealthcheckSpec{Type: "running", StableFor: time.Minute}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if time.Since(started) > 300*time.Millisecond {
		t.Fatalf("cancellation was not bounded: %s", time.Since(started))
	}
}

func TestAbsentHealthcheckUsesExistingDockerReadiness(t *testing.T) {
	engine := &fakeEngine{}
	runner := New(engine, nil)
	if err := runner.waitReady(context.Background(), "new", nil); err != nil {
		t.Fatal(err)
	}
	if engine.waitReadyCalls != 1 {
		t.Fatalf("WaitReady calls = %d", engine.waitReadyCalls)
	}
}

func TestReadinessFailureRestoresAllPreviousContainers(t *testing.T) {
	engine := &fakeEngine{
		exists:   map[string]bool{"uruflow-api-app": true, "uruflow-api-worker": true},
		endpoint: "127.0.0.1:1",
	}
	runner := New(engine, nil)
	request := ufp.ReleaseRequest{JobID: "r2", Project: "api", Services: []ufp.ServiceSpec{
		{Name: "app", Image: immutableTestImage, Labels: map[string]string{"traefik.enable": "true"}},
		{Name: "worker", Image: immutableTestImage, Healthcheck: &ufp.HealthcheckSpec{Type: "tcp", Port: 1, Interval: time.Millisecond, Timeout: 10 * time.Millisecond, Retries: 1}},
	}}
	if err := runner.Release(context.Background(), request, func(string, string) {}); err == nil {
		t.Fatal("failed readiness reported success")
	}
	if !engine.exists["uruflow-api-app"] || !engine.exists["uruflow-api-worker"] {
		t.Fatalf("previous containers were not restored: %v", engine.exists)
	}
	if len(engine.runSpecs) != 2 || engine.runSpecs[0].Labels["traefik.enable"] != "true" || engine.runSpecs[0].Labels[docker.LabelProject] != "api" {
		t.Fatalf("created specs = %+v", engine.runSpecs)
	}
}
