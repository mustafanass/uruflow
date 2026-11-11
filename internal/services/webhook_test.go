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
	"errors"
	"path/filepath"
	"testing"
	"time"

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

func TestWebhookMatchesOnlyEnabledSources(t *testing.T) {
	store, err := sqlite.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	const repository, commit = "git@github.com:acme/api.git", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := store.SaveProject(&models.Project{Name: "api", Services: []models.Service{
		{Name: "api", GitURL: repository, Branch: "main"},
		{Name: "worker", GitURL: repository, Branch: "develop"},
		{Name: "cache", Image: "redis:7"},
	}}); err != nil {
		t.Fatal(err)
	}
	service := NewWebhookService(store, nil)
	push := &Push{Repository: "api", Branch: "main", Commit: commit, Identities: []string{"https://github.com/acme/api.git"}}

	if matches, err := service.match(push); err != nil || len(matches) != 0 {
		t.Fatalf("matched a source that is not enabled: %+v %v", matches, err)
	}
	if err := store.SetPushSource(models.PushSource{Project: "api", Repository: repository, Branch: "main"}, true); err != nil {
		t.Fatal(err)
	}
	matches, err := service.match(push)
	if err != nil || len(matches) != 1 || len(matches["api"]) != 1 || matches["api"]["api"] != commit {
		t.Fatalf("matches = %+v %v, want only service api pinned to the pushed commit", matches, err)
	}
	push.Identities = []string{"https://github.com/other/api.git"}
	if matches, err := service.match(push); err != nil || len(matches) != 0 {
		t.Fatalf("matched an unrelated repository by name: %+v %v", matches, err)
	}
}

func TestEmptyWebhookSecretRejectsAuthentication(t *testing.T) {
	service := NewWebhookService(nil, nil)
	if service.VerifyGitHub([]byte("payload"), "") || service.VerifyGitLab("") {
		t.Fatal("an empty secret disabled webhook authentication")
	}
}

func TestRotatedWebhookSecretOverlapsOnlyUntilItExpires(t *testing.T) {
	service := NewWebhookService(nil, nil)
	service.Apply(true, "current-secret-value", "previous-secret-value", time.Now().Add(time.Hour))
	if !service.VerifyGitLab("current-secret-value") || !service.VerifyGitLab("previous-secret-value") {
		t.Fatal("a secret inside its overlap window was rejected")
	}
	service.Apply(true, "current-secret-value", "previous-secret-value", time.Now().Add(-time.Second))
	if !service.VerifyGitLab("current-secret-value") || service.VerifyGitLab("previous-secret-value") {
		t.Fatal("an expired previous secret was still accepted")
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

func TestIgnoredDeliveryKeepsItsRepositoryAndBranch(t *testing.T) {
	store, err := sqlite.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewWebhookService(store, nil)
	payload := []byte(`{"ref":"refs/heads/main","project":{"name":"api","git_http_url":"https://gitlab.com/acme/api.git"},"checkout_sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`)
	if claimed, err := service.ClaimDelivery(ProviderGitLab, "delivery-1"); err != nil || !claimed {
		t.Fatalf("claim = %v, %v", claimed, err)
	}
	result, err := service.Handle(ProviderGitLab, payload)
	if !errors.Is(err, ErrNoMatch) {
		t.Fatalf("handle = %v, want no match", err)
	}
	service.Record(ProviderGitLab, "delivery-1", result, err)
	deliveries, err := store.ListWebhookDeliveries(1)
	if err != nil || len(deliveries) != 1 {
		t.Fatalf("deliveries = %+v, %v", deliveries, err)
	}
	if delivery := deliveries[0]; delivery.Result != models.DeliveryIgnored || delivery.Repository != "api" || delivery.Branch != "main" || delivery.Detail == "" {
		t.Fatalf("ignored delivery lost why it was ignored: %+v", delivery)
	}
}

func TestDeliverySummaryReportsTheMostProgressedOutcome(t *testing.T) {
	for _, test := range []struct {
		outcomes []Outcome
		result   string
		detail   string
	}{
		{[]Outcome{{Project: "api", Queued: true}, {Project: "web"}}, models.DeliveryStarted, "api queued, web started"},
		{[]Outcome{{Project: "api", Err: errors.New("busy")}, {Project: "web", Queued: true}}, models.DeliveryQueued, "api failed: busy, web queued"},
		{[]Outcome{{Project: "api", Err: errors.New("busy")}}, models.DeliveryFailed, "api failed: busy"},
	} {
		if result, detail := summarize(test.outcomes); result != test.result || detail != test.detail {
			t.Errorf("summarize(%+v) = %q %q, want %q %q", test.outcomes, result, detail, test.result, test.detail)
		}
	}
}
