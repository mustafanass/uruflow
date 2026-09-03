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

func TestVariableEditorSeparatesPlainAndSecretValues(t *testing.T) {
	raw := "# app settings\nLOG_LEVEL=info\nDATABASE_URL=${secret:legacy.db}\n"
	editor, err := FormatVariableEditor(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(editor, "LOG_LEVEL=info") || !strings.Contains(editor, "secret DATABASE_URL=${secret:legacy.db}") {
		t.Fatalf("editor =\n%s", editor)
	}
	unchanged, err := ParseVariableEditor(editor, "api-prod")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.DotEnv != raw || len(unchanged.Secrets) != 0 {
		t.Fatalf("unchanged = %#v", unchanged)
	}
}

func TestVariableEditorTurnsNewSecretValuesIntoReferences(t *testing.T) {
	input := "LOG_LEVEL=debug\nsecret DATABASE_URL='postgres://db?a=1'\nsecret API_TOKEN=value=with=equals\n"
	edit, err := ParseVariableEditor(input, "api-prod")
	if err != nil {
		t.Fatal(err)
	}
	if edit.Secrets["api-prod.DATABASE_URL"] != "postgres://db?a=1" || edit.Secrets["api-prod.API_TOKEN"] != "value=with=equals" {
		t.Fatalf("secrets = %#v", edit.Secrets)
	}
	if !strings.Contains(edit.DotEnv, "DATABASE_URL=${secret:api-prod.DATABASE_URL}") || strings.Contains(edit.DotEnv, "postgres://") {
		t.Fatalf("dotenv =\n%s", edit.DotEnv)
	}
}

func TestVariableEditorRejectsDuplicateAndEmptySecrets(t *testing.T) {
	for _, input := range []string{
		"MODE=prod\nsecret MODE=value\n",
		"secret TOKEN=\n",
		"BAD-NAME=value\n",
	} {
		if _, err := ParseVariableEditor(input, "api-prod"); err == nil {
			t.Fatalf("input unexpectedly accepted: %q", input)
		}
	}
}

func TestEmptyVariableEditorClearsTheEnvironment(t *testing.T) {
	edit, err := ParseVariableEditor("", "api-prod")
	if err != nil {
		t.Fatal(err)
	}
	if edit.DotEnv != "" || len(edit.Secrets) != 0 {
		t.Fatalf("edit = %#v", edit)
	}
}
