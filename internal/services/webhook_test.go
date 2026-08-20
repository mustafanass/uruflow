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
	"path/filepath"
	"testing"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
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

func TestWebhookDoesNotMatchAProjectByNameAlone(t *testing.T) {
	store, err := sqlite.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveProject(&models.Project{
		Name: "api", GitURL: "https://github.com/other/api.git", Branch: "main", AutoDeploy: true,
	}); err != nil {
		t.Fatal(err)
	}
	service := NewWebhookService(config.Default(), store, nil)
	matches, err := service.match(&Push{
		Repository: "api", Branch: "main", Identities: []string{"https://github.com/acme/api.git"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matched unrelated repository: %+v", matches)
	}
}

func TestEmptyWebhookSecretRejectsAuthentication(t *testing.T) {
	cfg := config.Default()
	cfg.Webhook.Secret = ""
	service := NewWebhookService(cfg, nil, nil)
	if service.VerifyGitHub([]byte("payload"), "") || service.VerifyGitLab("") {
		t.Fatal("an empty secret disabled webhook authentication")
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
			"head_commit": {"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	}`)

	push, err := parsePush(ProviderGitHub, payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if push.Branch != "develop" || push.Repository != "api" || push.Commit != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
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
	payload := []byte(`{"ref":"refs/tags/v1","repository":{"name":"api"},"head_commit":{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)

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
		"checkout_sha": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	}`)

	push, err := parsePush(ProviderGitLab, payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if push.Branch != "main" || push.Commit != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("push = %+v", push)
	}
	if !sameRepository("https://gitlab.com/acme/api", push.Identities) {
		t.Error("gitlab identities did not match")
	}
}
