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

package version

import "testing"

func TestAtLeastComparesReleaseNumbers(t *testing.T) {
	for _, test := range []struct {
		current, minimum string
		want             bool
	}{
		{"3.0.6", "3.0.6", true},
		{"3.0.10", "3.0.6", true},
		{"3.1.0", "3.0.6", true},
		{"v3.0.6", "3.0.6", true},
		{"3.0.5", "3.0.6", false},
		{"2.9.9", "3.0.6", false},
		{"", "3.0.6", false},
		{"3.0", "3.0.6", false},
		{"dev", "3.0.6", false},
	} {
		if got := AtLeast(test.current, test.minimum); got != test.want {
			t.Errorf("AtLeast(%q, %q) = %v, want %v", test.current, test.minimum, got, test.want)
		}
	}
}
