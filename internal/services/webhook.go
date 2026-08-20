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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/pipeline"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const (
	ProviderGitHub = "github"
	ProviderGitLab = "gitlab"

	signaturePrefix = "sha256="
	branchPrefix    = "refs/heads/"
	shortCommitSize = 7
)

var (
	ErrUnknownProvider = errors.New("unrecognised webhook provider")
	ErrNoMatch         = errors.New("no project is wired to this repository and branch")
)

type WebhookService struct {
	cfg      *config.Config
	store    storage.Store
	pipeline *pipeline.Pipeline
}

type Push struct {
	Repository string
	Identities []string
	Branch     string
	Commit     string
}

type Outcome struct {
	Project string
	Release string
	Err     error
}

type Result struct {
	Push
	Outcomes []Outcome
}

func NewWebhookService(cfg *config.Config, store storage.Store, releases *pipeline.Pipeline) *WebhookService {
	return &WebhookService{cfg: cfg, store: store, pipeline: releases}
}

func (s *WebhookService) Handle(provider string, payload []byte) (*Result, error) {
	push, err := parsePush(provider, payload)
	if err != nil {
		return nil, err
	}

	matches, err := s.match(push)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s on %s: %w", push.Repository, push.Branch, ErrNoMatch)
	}

	logger.Info("[WEBHOOK] %s pushed %s@%s on %s, matching %d project(s)",
		provider, push.Repository, shortCommit(push.Commit), push.Branch, len(matches))

	result := &Result{Push: *push}
	for _, project := range matches {
		outcome := Outcome{Project: project.Name}

		release, err := s.pipeline.Trigger(project.Name, push.Commit, models.TriggerWebhook)
		if err != nil {
			outcome.Err = err
			logger.Warn("[WEBHOOK] %s: %v", project.Name, err)
		} else {
			outcome.Release = release.ID
		}

		result.Outcomes = append(result.Outcomes, outcome)
	}

	return result, nil
}

func (s *WebhookService) match(push *Push) ([]models.Project, error) {
	projects, err := s.store.ListProjects()
	if err != nil {
		return nil, err
	}

	matches := make([]models.Project, 0, 1)
	for _, project := range projects {
		if !project.AutoDeploy || project.Branch != push.Branch {
			continue
		}
		if sameRepository(project.GitURL, push.Identities) || project.Name == push.Repository {
			matches = append(matches, project)
		}
	}

	return matches, nil
}

func (s *WebhookService) VerifyGitHub(payload []byte, signature string) bool {
	if s.cfg.Webhook.Secret == "" {
		return true
	}
	if !strings.HasPrefix(signature, signaturePrefix) {
		return false
	}

	mac := hmac.New(sha256.New, []byte(s.cfg.Webhook.Secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(strings.TrimPrefix(signature, signaturePrefix)), []byte(expected))
}

func (s *WebhookService) VerifyGitLab(token string) bool {
	if s.cfg.Webhook.Secret == "" {
		return true
	}
	return hmac.Equal([]byte(token), []byte(s.cfg.Webhook.Secret))
}

func parsePush(provider string, payload []byte) (*Push, error) {
	switch provider {
	case ProviderGitHub:
		var body struct {
			Ref        string `json:"ref"`
			Repository struct {
				Name     string `json:"name"`
				FullName string `json:"full_name"`
				CloneURL string `json:"clone_url"`
				SSHURL   string `json:"ssh_url"`
				HTMLURL  string `json:"html_url"`
			} `json:"repository"`
			HeadCommit struct {
				ID string `json:"id"`
			} `json:"head_commit"`
		}
		if err := json.Unmarshal(payload, &body); err != nil {
			return nil, fmt.Errorf("parse github payload: %w", err)
		}
		return newPush(body.Repository.Name, body.Ref, body.HeadCommit.ID, []string{
			body.Repository.CloneURL, body.Repository.SSHURL,
			body.Repository.HTMLURL, body.Repository.FullName,
		})

	case ProviderGitLab:
		var body struct {
			Ref     string `json:"ref"`
			Project struct {
				Name              string `json:"name"`
				PathWithNamespace string `json:"path_with_namespace"`
				GitSSHURL         string `json:"git_ssh_url"`
				GitHTTPURL        string `json:"git_http_url"`
				WebURL            string `json:"web_url"`
			} `json:"project"`
			CheckoutSHA string `json:"checkout_sha"`
		}
		if err := json.Unmarshal(payload, &body); err != nil {
			return nil, fmt.Errorf("parse gitlab payload: %w", err)
		}
		return newPush(body.Project.Name, body.Ref, body.CheckoutSHA, []string{
			body.Project.GitSSHURL, body.Project.GitHTTPURL,
			body.Project.WebURL, body.Project.PathWithNamespace,
		})
	}

	return nil, ErrUnknownProvider
}

func newPush(repository, ref, commit string, identities []string) (*Push, error) {
	branch := strings.TrimPrefix(ref, branchPrefix)
	if branch == ref || branch == "" {
		return nil, fmt.Errorf("unsupported git ref %q", ref)
	}
	if repository == "" {
		return nil, errors.New("webhook payload carries no repository name")
	}

	kept := make([]string, 0, len(identities))
	for _, identity := range identities {
		if identity != "" {
			kept = append(kept, identity)
		}
	}

	return &Push{Repository: repository, Identities: kept, Branch: branch, Commit: commit}, nil
}

func shortCommit(commit string) string {
	if len(commit) <= shortCommitSize {
		return commit
	}
	return commit[:shortCommitSize]
}
