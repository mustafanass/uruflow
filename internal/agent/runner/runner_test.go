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
	"testing"

	"github.com/mustafanass/uruflow/internal/docker"
)

func TestCredentialsOnlyGoToTheUruflowRegistry(t *testing.T) {
	runner := New(nil, func() *docker.Auth {
		return &docker.Auth{Username: "uruflow", Password: "secret", ServerAddress: "reg.internal:5000"}
	})

	cases := map[string]bool{
		"reg.internal:5000/uruflow/api:abc": true,
		"reg.internal:5000/uruflow/api":     true,
		"redis:7-alpine":                    false,
		"docker.io/library/redis:7":         false,
		"ghcr.io/acme/thing:1":              false,
		"reg.internal:5001/uruflow/api:abc": false,
		"evil-reg.internal:5000/x/y":        false,
	}

	for image, want := range cases {
		got := runner.authFor(image) != nil
		if got != want {
			t.Errorf("%s: credentials attached = %v, want %v", image, got, want)
		}
	}
}

func TestNoCredentialsWhenTheRegistryIsUnconfigured(t *testing.T) {
	runner := New(nil, func() *docker.Auth { return nil })

	if runner.authFor("reg.internal:5000/uruflow/api") != nil {
		t.Error("credentials were invented without a configured registry")
	}
}

func TestContainerNaming(t *testing.T) {
	if name := ContainerName("api-prod", ""); name != "uruflow-api-prod" {
		t.Errorf("single-service name = %q", name)
	}
	if name := ContainerName("api-prod", "worker"); name != "uruflow-api-prod-worker" {
		t.Errorf("multi-service name = %q", name)
	}
}
