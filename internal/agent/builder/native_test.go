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

package builder

import (
	"testing"

	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestTargetSourceUsesOnlyTheServiceSource(t *testing.T) {
	target := ufp.BuildTarget{
		GitURL: "git@example/api.git", Branch: "main", Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	gitURL, branch, commit := targetSource(target)
	if gitURL != target.GitURL || branch != target.Branch || commit != target.Commit {
		t.Fatalf("service source = %q %q %q", gitURL, branch, commit)
	}
}
