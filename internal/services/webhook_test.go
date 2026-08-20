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

package services

import (
	"testing"
)

func TestNormalizeGitURLTreatsEveryFormAsOneRepository(t *testing.T) {
	forms := []string{
		"git@github.com:acme/api.git",
		"https://github.com/acme/api.git",
		"https://github.com/acme/api",
		"ssh://git@github.com/acme/api.git",
		"git://github.com/acme/api.git",
		"HTTPS://GitHub.com/Acme/API.git",
	}

	want := normalizeGitURL(forms[0])
	if want != "github.com/acme/api" {
		t.Fatalf("normalised form = %q", want)
	}

	for _, form := range forms {
		if got := normalizeGitURL(form); got != want {
			t.Errorf("%q normalised to %q, want %q", form, got, want)
		}
	}
}

func TestSameRepositoryRejectsDifferentRepos(t *testing.T) {
	identities := []string{"https://github.com/acme/api.git", "acme/api"}

	if !sameRepository("git@github.com:acme/api.git", identities) {
		t.Error("the same repository in another form was not matched")
	}
	if sameRepository("git@github.com:acme/web.git", identities) {
		t.Error("a different repository was matched")
	}
	if sameRepository("", identities) {
		t.Error("an empty git url matched")
	}
}

func TestParsePushCollectsEveryIdentity(t *testing.T) {
	payload := []byte(`{
		"ref": "refs/heads/develop",
		"repository": {
			"name": "api",
			"full_name": "acme/api",
			"clone_url": "https://github.com/acme/api.git",
			"ssh_url": "git@github.com:acme/api.git"
		},
		"head_commit": {"id": "abc123"}
	}`)

	push, err := parsePush(ProviderGitHub, payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if push.Branch != "develop" || push.Repository != "api" || push.Commit != "abc123" {
		t.Fatalf("push = %+v", push)
	}
	for _, form := range []string{
		"git@github.com:acme/api.git",
		"https://github.com/acme/api",
		"acme/api",
	} {
		if !sameRepository(form, push.Identities) {
			t.Errorf("%q did not match the push identities %v", form, push.Identities)
		}
	}
	for _, identity := range push.Identities {
		if identity == "" {
			t.Error("an empty identity was kept")
		}
	}
}

func TestParsePushRejectsTags(t *testing.T) {
	payload := []byte(`{"ref":"refs/tags/v1","repository":{"name":"api"},"head_commit":{"id":"a"}}`)

	if _, err := parsePush(ProviderGitHub, payload); err == nil {
		t.Error("a tag push should not be treated as a branch push")
	}
}

func TestParsePushReadsGitLab(t *testing.T) {
	payload := []byte(`{
		"ref": "refs/heads/main",
		"project": {
			"name": "api",
			"path_with_namespace": "acme/api",
			"git_ssh_url": "git@gitlab.com:acme/api.git",
			"git_http_url": "https://gitlab.com/acme/api.git"
		},
		"checkout_sha": "def456"
	}`)

	push, err := parsePush(ProviderGitLab, payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if push.Branch != "main" || push.Commit != "def456" {
		t.Fatalf("push = %+v", push)
	}
	if !sameRepository("https://gitlab.com/acme/api", push.Identities) {
		t.Error("gitlab identities did not match")
	}
}
