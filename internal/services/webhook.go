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
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

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
	store    storage.Store
	pipeline *pipeline.Pipeline

	mu            sync.RWMutex
	enabled       bool
	secret        string
	previous      string
	previousUntil time.Time
	rejection     Rejection
}

type Rejection struct {
	At       time.Time `json:"at"`
	Provider string    `json:"provider"`
	Reason   string    `json:"reason"`
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
	Queued  bool
	Err     error
}

type Result struct {
	Push
	Outcomes []Outcome
}

func NewWebhookService(store storage.Store, releases *pipeline.Pipeline) *WebhookService {
	return &WebhookService{store: store, pipeline: releases}
}

func (s *WebhookService) Apply(enabled bool, secret, previous string, previousUntil time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	s.secret = secret
	s.previous = previous
	s.previousUntil = previousUntil
}

func (s *WebhookService) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *WebhookService) Reject(provider, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejection = Rejection{At: time.Now(), Provider: provider, Reason: reason}
}

func (s *WebhookService) LastRejection() (Rejection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rejection, !s.rejection.At.IsZero()
}

func (s *WebhookService) Record(provider, deliveryID string, result *Result, failure error) {
	delivery := models.WebhookDelivery{Provider: provider, DeliveryID: deliveryID}
	if result != nil {
		delivery.Repository, delivery.Branch, delivery.Commit = result.Repository, result.Branch, result.Commit
	}
	switch {
	case errors.Is(failure, ErrNoMatch):
		delivery.Result, delivery.Detail = models.DeliveryIgnored, failure.Error()
	case failure != nil:
		delivery.Result, delivery.Detail = models.DeliveryFailed, failure.Error()
	default:
		delivery.Result, delivery.Detail = summarize(result.Outcomes)
	}
	if err := s.store.RecordWebhookDelivery(delivery); err != nil {
		logger.Warn("[WEBHOOK] record delivery %s: %v", deliveryID, err)
	}
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
		return &Result{Push: *push}, fmt.Errorf("%s on %s: %w", push.Repository, push.Branch, ErrNoMatch)
	}

	logger.Info("[WEBHOOK] %s pushed %s@%s on %s, matching %d project(s)",
		provider, push.Repository, shortCommit(push.Commit), push.Branch, len(matches))

	result := &Result{Push: *push}
	for _, name := range slices.Sorted(maps.Keys(matches)) {
		outcome := Outcome{Project: name}

		release, err := s.pipeline.Push(name, matches[name])
		switch {
		case errors.Is(err, pipeline.ErrQueued):
			outcome.Queued = true
		case err != nil:
			outcome.Err = err
			logger.Warn("[WEBHOOK] %s: %v", name, err)
		default:
			outcome.Release = release.ID
		}

		result.Outcomes = append(result.Outcomes, outcome)
	}

	return result, nil
}

func (s *WebhookService) match(push *Push) (map[string]map[string]string, error) {
	sources, err := s.store.ListPushSources()
	if err != nil {
		return nil, err
	}

	matches := make(map[string]map[string]string)
	for _, source := range sources {
		if source.Branch != push.Branch || !sameRepository(source.Repository, push.Identities) {
			continue
		}
		project, err := s.store.GetProject(source.Project)
		if err != nil {
			return nil, err
		}
		for _, service := range project.ServiceList() {
			if !service.Built() || service.GitURL != source.Repository || service.Branch != source.Branch {
				continue
			}
			if matches[project.Name] == nil {
				matches[project.Name] = make(map[string]string)
			}
			matches[project.Name][service.Name] = push.Commit
		}
	}

	return matches, nil
}

func summarize(outcomes []Outcome) (string, string) {
	result := models.DeliveryFailed
	details := make([]string, 0, len(outcomes))
	for _, outcome := range outcomes {
		switch {
		case outcome.Err != nil:
			details = append(details, outcome.Project+" failed: "+outcome.Err.Error())
		case outcome.Queued:
			details = append(details, outcome.Project+" queued")
			if result == models.DeliveryFailed {
				result = models.DeliveryQueued
			}
		default:
			details = append(details, outcome.Project+" started")
			result = models.DeliveryStarted
		}
	}
	return result, strings.Join(details, ", ")
}

func (s *WebhookService) VerifyGitHub(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, signaturePrefix) {
		return false
	}

	for _, secret := range s.secrets() {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expected := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(strings.TrimPrefix(signature, signaturePrefix)), []byte(expected)) {
			return true
		}
	}
	return false
}

func (s *WebhookService) VerifyGitLab(token string) bool {
	for _, secret := range s.secrets() {
		if hmac.Equal([]byte(token), []byte(secret)) {
			return true
		}
	}
	return false
}

func (s *WebhookService) secrets() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	secrets := make([]string, 0, 2)
	if s.secret != "" {
		secrets = append(secrets, s.secret)
	}
	if s.previous != "" && time.Now().Before(s.previousUntil) {
		secrets = append(secrets, s.previous)
	}
	return secrets
}

func (s *WebhookService) ClaimDelivery(provider, deliveryID string) (bool, error) {
	return s.store.ClaimWebhookDelivery(provider, deliveryID)
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
	if !models.ValidGitCommit(commit) {
		return nil, errors.New("webhook payload carries no valid commit")
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
