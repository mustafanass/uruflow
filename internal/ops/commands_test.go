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

package ops

import "testing"

func TestAgentRolesAreExplicitAndCanonical(t *testing.T) {
	for _, test := range []struct {
		options []string
		want    string
	}{
		{want: "runner"},
		{options: []string{"--roles", "builder"}, want: "builder"},
		{options: []string{"--roles", "builder,runner"}, want: "builder,runner"},
	} {
		_, got, err := parseAgentRoles(test.options)
		if err != nil {
			t.Fatalf("parse %v: %v", test.options, err)
		}
		if got != test.want {
			t.Fatalf("parse %v = %q, want %q", test.options, got, test.want)
		}
	}

	for _, options := range [][]string{{"builder"}, {"--role", "builder"}, {"--roles"}, {"--roles", "builder", "extra"}} {
		if _, _, err := parseAgentRoles(options); err == nil {
			t.Fatalf("parse %v unexpectedly succeeded", options)
		}
	}
}
