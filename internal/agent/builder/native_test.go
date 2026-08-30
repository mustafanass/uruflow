/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 * This file is part of uruflow and is licensed under the MIT License.
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
