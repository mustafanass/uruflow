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

package docker

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestStateFailedDetectsTerminalContainers(t *testing.T) {
	cases := []struct {
		name  string
		state State
		fails bool
	}{
		{"crashed on boot", State{Status: StateExited, ExitCode: 1, Health: HealthNone}, true},
		{"clean exit still counts as gone", State{Status: StateExited, Health: HealthNone}, true},
		{"dead", State{Status: StateDead, Health: HealthNone}, true},
		{"healthcheck failed", State{Status: StateRunning, Health: HealthUnhealthy}, true},
		{"still starting", State{Status: StateRunning, Health: HealthStarting}, false},
		{"restarting", State{Status: StateRestarting, Health: HealthNone}, false},
		{"healthy", State{Status: StateRunning, Health: HealthHealthy}, false},
		{"running without a healthcheck", State{Status: StateRunning, Health: HealthNone}, false},
	}

	for _, test := range cases {
		if failed := test.state.Failed() != nil; failed != test.fails {
			t.Errorf("%s: Failed() = %v, want %v", test.name, failed, test.fails)
		}
	}
}

func TestStateReadyOnlyWhenRunning(t *testing.T) {
	cases := []struct {
		name  string
		state State
		ready bool
	}{
		{"healthy", State{Status: StateRunning, Health: HealthHealthy}, true},
		{"running without a healthcheck", State{Status: StateRunning, Health: HealthNone}, true},
		{"still starting", State{Status: StateRunning, Health: HealthStarting}, false},
		{"unhealthy", State{Status: StateRunning, Health: HealthUnhealthy}, false},
		{"restarting", State{Status: StateRestarting, Health: HealthNone}, false},
		{"exited", State{Status: StateExited, Health: HealthNone}, false},
	}

	for _, test := range cases {
		if ready := test.state.Ready(); ready != test.ready {
			t.Errorf("%s: Ready() = %v, want %v", test.name, ready, test.ready)
		}
	}
}

func TestDigestReferenceIsSplitForPull(t *testing.T) {
	repository, digest := SplitTag("registry.example/api@sha256:abc")
	if repository != "registry.example/api" || digest != "sha256:abc" {
		t.Fatalf("split = %q, %q", repository, digest)
	}
}

func TestPullRejectsMalformedProgress(t *testing.T) {
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":"pulling"`)),
			Header:     make(http.Header),
		}, nil
	})}}
	if err := client.Pull(context.Background(), "registry.example/api@sha256:abc", nil, nil); err == nil {
		t.Fatal("malformed progress stream was accepted")
	}
}

func TestContainerLabelsPreserveUserValuesAndProtectOwnership(t *testing.T) {
	labels, err := ContainerLabels(map[string]string{"traefik.enable": "true", "empty": ""}, "api", "web", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if labels["traefik.enable"] != "true" || labels["empty"] != "" || labels[LabelProject] != "api" || labels[LabelService] != "web" {
		t.Fatalf("labels = %#v", labels)
	}
	if _, err := ContainerLabels(map[string]string{LabelProject: "forged"}, "api", "", "r1"); err == nil {
		t.Fatal("reserved label was accepted")
	}
}

func TestEndpointPrefersPublishedPortThenContainerAddress(t *testing.T) {
	response := `{"NetworkSettings":{"Ports":{"8080/tcp":[{"HostIp":"0.0.0.0","HostPort":"18080"}]},"Networks":{"bridge":{"IPAddress":"172.18.0.4"}}}}`
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	})}}
	endpoint, err := client.Endpoint(context.Background(), "container", 8080)
	if err != nil || endpoint != "127.0.0.1:18080" {
		t.Fatalf("published endpoint = %q err=%v", endpoint, err)
	}

	response = `{"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.18.0.4"}}}}`
	endpoint, err = client.Endpoint(context.Background(), "container", 8080)
	if err != nil || endpoint != "172.18.0.4:8080" {
		t.Fatalf("container endpoint = %q err=%v", endpoint, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
