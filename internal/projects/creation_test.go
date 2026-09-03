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

package projects

import (
	"strings"
	"testing"
)

func TestCreationYAMLIsStrictAndValidatesTheEnvironment(t *testing.T) {
	environment, err := ParseCreationYAML(`workflow: build_deploy
builder: builder-01
runners: [dev-01]
services:
  api:
    git: https://github.com/example/api.git
    branch: main
    dockerfile: Dockerfile
    context: .
`)
	if err != nil {
		t.Fatal(err)
	}
	if environment.Services["api"].Git != "https://github.com/example/api.git" || environment.Services["api"].Dockerfile != "Dockerfile" {
		t.Fatalf("environment = %+v", environment)
	}

	_, err = ParseCreationYAML("workflo: build_deploy\n")
	if err == nil || !strings.Contains(err.Error(), "workflo") {
		t.Fatalf("unknown field error = %v", err)
	}
}
