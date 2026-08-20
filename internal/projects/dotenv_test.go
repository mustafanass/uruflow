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

import "testing"

func TestParseDotEnvHandlesRealFiles(t *testing.T) {
	content := `
# a comment
export MODE=production

QUOTED="hello world"
SINGLE='keep #this'
ESCAPED="line\nbreak"
EMPTY=
URL=postgres://user:pass@host:5432/db?sslmode=disable
SPACED  =  trimmed
`

	values, err := ParseDotEnv(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	cases := map[string]string{
		"MODE":    "production",
		"QUOTED":  "hello world",
		"SINGLE":  "keep #this",
		"ESCAPED": "line\nbreak",
		"EMPTY":   "",
		"URL":     "postgres://user:pass@host:5432/db?sslmode=disable",
		"SPACED":  "trimmed",
	}
	for key, want := range cases {
		if got := values[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestParseDotEnvRejectsBadLines(t *testing.T) {
	cases := map[string]string{
		"no equals sign":    "JUST_A_NAME\n",
		"empty name":        "=value\n",
		"invalid name":      "BAD-NAME=1\n",
		"name starts digit": "1BAD=1\n",
	}

	for name, content := range cases {
		if _, err := ParseDotEnv(content); err == nil {
			t.Errorf("%s: expected a rejection", name)
		}
	}
}
