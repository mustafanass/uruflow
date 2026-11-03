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

func TestBuildArgumentsRejectSecretReferencesAfterInterpolation(t *testing.T) {
	base := "builder: build-01\nrunners: [run-01]\nenv:\n  CREDENTIAL: '${secret:app.TOKEN}'\n  MODE: production\nservices:\n  api:\n    git: git@host:app.git\n    branch: main\n    build_args:\n      VALUE: "
	for _, value := range []string{"${secret:app.TOKEN}", "prefix-${secret:app.TOKEN}", "${CREDENTIAL}"} {
		if _, err := ParseCreationYAML(base + "'" + value + "'\n"); err == nil || !strings.Contains(err.Error(), "build_args does not support secrets") {
			t.Fatalf("secret argument accepted: %q %v", value, err)
		}
	}
	if _, err := ParseCreationYAML(base + "'${MODE}'\n"); err != nil {
		t.Fatal("normal build argument rejected", err)
	}
}
