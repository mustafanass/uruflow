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

package daemon

import (
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestBuildRequestAcceptsServiceOwnedSourcesWithoutAPrimary(t *testing.T) {
	request := ufp.BuildRequest{
		JobID: "r1", Project: "urufi-prod",
		Targets: []ufp.BuildTarget{{
			Service: "core", Image: "registry/urufi-core:latest", Dockerfile: "Dockerfile", Context: ".",
			GitURL: "git@example/core.git", Branch: "main",
		}},
	}
	if err := validateBuildRequest(request); err != nil {
		t.Fatalf("service-owned source rejected: %v", err)
	}

	request.Targets[0].Branch = ""
	if err := validateBuildRequest(request); err == nil {
		t.Fatal("incomplete service-owned source was accepted")
	}
}

func TestReleaseRequestValidatesHealthchecksAndLabels(t *testing.T) {
	request := ufp.ReleaseRequest{JobID: "r1", Project: "api", Services: []ufp.ServiceSpec{{
		Image:       "repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Healthcheck: &ufp.HealthcheckSpec{Type: "http", Scheme: "http", Path: "/ready", Port: 8080, Interval: time.Second, Timeout: time.Second, Retries: 3},
		Labels:      map[string]string{"traefik.enable": "true"},
	}}}
	if err := validateReleaseRequest(request); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	request.Services[0].Healthcheck.Type = "exec"
	if err := validateReleaseRequest(request); err == nil {
		t.Fatal("unknown healthcheck type was accepted")
	}
	request.Services[0].Healthcheck.Type = "http"
	request.Services[0].Labels = map[string]string{"uruflow.project": "forged"}
	if err := validateReleaseRequest(request); err == nil {
		t.Fatal("reserved label was accepted")
	}
}
