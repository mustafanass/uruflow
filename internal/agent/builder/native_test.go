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

func TestTargetSourceDoesNotReusePrimaryCommitForSecondaryRepository(t *testing.T) {
	request := ufp.BuildRequest{GitURL: "git@example/primary.git", Branch: "main", Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	gitURL, branch, commit, primary := targetSource(request, ufp.BuildTarget{GitURL: "git@example/api.git", Branch: "release"})
	if gitURL != "git@example/api.git" || branch != "release" || commit != "" || primary {
		t.Fatalf("secondary source = %q %q %q primary=%t", gitURL, branch, commit, primary)
	}

	_, _, commit, primary = targetSource(request, ufp.BuildTarget{})
	if commit != request.Commit || !primary {
		t.Fatalf("primary commit = %q primary=%t", commit, primary)
	}
}
