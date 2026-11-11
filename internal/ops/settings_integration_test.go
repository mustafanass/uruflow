//go:build integration

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

package ops

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/pipeline"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	legacyRegistryPassword = "legacy-registry-password"
	legacyWebhookSecret    = "legacy-webhook-secret-longer-than-32-characters"
)

func legacyConfig(t *testing.T) *config.Config {
	t.Helper()
	if os.Getenv("URUFLOW_DOCKER_TESTS") == "" {
		t.Skip("set URUFLOW_DOCKER_TESTS=1 for Docker discovery")
	}
	retain := 3
	cfg := config.Default()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.DataDir = t.TempDir()
	cfg.Server.Advertise = "localhost"
	cfg.Registry.Host = "localhost"
	cfg.Registry.Password = legacyRegistryPassword
	cfg.Registry.RetainReleases = &retain
	cfg.Webhook.Secret = legacyWebhookSecret
	cfg.Webhook.Path = "/hooks"
	if err := cfg.Save(filepath.Join(cfg.Server.DataDir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func settingsEngine(t *testing.T, cfg *config.Config) (*Engine, *sqlite.Store) {
	t.Helper()
	store, err := sqlite.New(cfg.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	server, err := api.NewServer(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Shutdown(context.Background()) })
	return New(server), store
}

func registryPassword(t *testing.T, engine *Engine) string {
	t.Helper()
	_, settings, err := engine.server.RegistrySettings()
	if err != nil {
		t.Fatal(err)
	}
	password, err := engine.server.Vault().Open(settings.Password)
	if err != nil {
		t.Fatal(err)
	}
	return password
}

func TestLegacySettingsAreImportedOnceAndStayEncrypted(t *testing.T) {
	cfg := legacyConfig(t)
	engine, store := settingsEngine(t, cfg)
	registry, err := engine.RegistrySettings()
	if err != nil || registry.RetainReleases != 3 || registry.Revision != 1 {
		t.Fatalf("registry import = %+v %v", registry, err)
	}
	webhook, err := engine.WebhookSettings()
	if err != nil || !webhook.Enabled || !webhook.TLS || webhook.Path != "/hooks" || webhook.Port != config.DefaultWebhookPort || webhook.SecretHint != legacyWebhookSecret[len(legacyWebhookSecret)-4:] {
		t.Fatalf("webhook import = %+v %v", webhook, err)
	}
	if password := registryPassword(t, engine); password != legacyRegistryPassword {
		t.Fatal("the agents' existing registry login was not kept")
	}
	for _, name := range []string{models.SettingRegistry, models.SettingWebhook} {
		setting, err := store.GetSetting(name)
		if err != nil || strings.Contains(setting.Value, legacyRegistryPassword) || strings.Contains(setting.Value, legacyWebhookSecret) {
			t.Fatalf("%s settings hold a plaintext credential: %v", name, err)
		}
	}

	*cfg.Registry.RetainReleases = 7
	cfg.Registry.Password = "another-legacy-registry-password"
	restarted, _ := settingsEngine(t, cfg)
	registry, err = restarted.RegistrySettings()
	if err != nil || registry.RetainReleases != 3 || registry.Revision != 1 || registryPassword(t, restarted) != legacyRegistryPassword {
		t.Fatalf("settings were imported again after a restart: %+v %v", registry, err)
	}
}

func TestSettingsRejectStaleAndUnsafeChanges(t *testing.T) {
	engine, store := settingsEngine(t, legacyConfig(t))
	webhook, err := engine.WebhookSettings()
	if err != nil {
		t.Fatal(err)
	}
	cfg := engine.server.Config()
	for _, change := range []WebhookChange{
		{Port: cfg.Web.Port, Path: "/hooks", TLS: true},
		{Port: config.DefaultWebhookPort, Path: "/api/hooks", TLS: true},
		{Port: config.DefaultWebhookPort, Path: "/health", TLS: true},
		{Port: config.DefaultWebhookPort, Path: "/hooks/{id}", TLS: true},
		{Host: "uruflow.example", Port: config.DefaultWebhookPort, Path: "/hooks", TLS: true},
		{Host: "0.0.0.0", Port: config.DefaultWebhookPort, Path: "/hooks"},
	} {
		change.Revision = webhook.Revision
		if _, err := engine.SaveWebhookSettings(change); err == nil {
			t.Fatalf("unsafe webhook accepted: %+v", change)
		}
	}
	if _, err := engine.SaveWebhookSettings(WebhookChange{Revision: webhook.Revision + 1, Port: config.DefaultWebhookPort, Path: "/hooks", TLS: true}); !errors.Is(err, ErrSettingsConflict) {
		t.Fatalf("stale webhook change = %v", err)
	}

	registry, err := engine.RegistrySettings()
	if err != nil {
		t.Fatal(err)
	}
	for _, retain := range []int{-1, models.MaxRetainReleases + 1} {
		if _, err := engine.SaveRegistrySettings(RegistryChange{Revision: registry.Revision, RetainReleases: retain}); err == nil {
			t.Fatalf("cleanup window %d accepted", retain)
		}
	}
	saved, err := engine.SaveRegistrySettings(RegistryChange{Revision: registry.Revision})
	if err != nil || saved.RetainReleases != 0 || saved.Revision != registry.Revision+1 {
		t.Fatalf("turn cleanup off = %+v %v", saved, err)
	}
	if _, err := engine.SaveRegistrySettings(RegistryChange{Revision: registry.Revision, RetainReleases: 5}); !errors.Is(err, ErrSettingsConflict) {
		t.Fatalf("stale registry change = %v", err)
	}
	if err := store.CreateRelease(&models.Release{ID: "active", Project: "p", Status: models.StatusBuilding, StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.RotateRegistryPassword(saved.Revision); !errors.Is(err, pipeline.ErrRegistryBusy) {
		t.Fatalf("rotation during a release = %v", err)
	}
	if registryPassword(t, engine) != legacyRegistryPassword {
		t.Fatal("a refused rotation changed the registry password")
	}

	reporting, err := engine.AgentReporting()
	if err != nil || reporting.Managed || reporting.MetricsSec != ufp.DefaultMetricsSec || reporting.ReconnectSec != ufp.DefaultReconnectSec {
		t.Fatalf("unmanaged reporting = %+v %v", reporting, err)
	}
	for _, change := range []AgentReportingChange{
		{MetricsSec: ufp.MinMetricsSec - 1, ReconnectSec: ufp.DefaultReconnectSec},
		{MetricsSec: ufp.DefaultMetricsSec, ReconnectSec: ufp.MaxReconnectSec + 1},
	} {
		if _, err := engine.SaveAgentReporting(change); err == nil {
			t.Fatalf("reporting %+v accepted", change)
		}
	}
	reporting, err = engine.SaveAgentReporting(AgentReportingChange{MetricsSec: 30, ReconnectSec: 10})
	if err != nil || !reporting.Managed || reporting.Revision != 1 {
		t.Fatalf("manage reporting = %+v %v", reporting, err)
	}
	if _, err := engine.SaveAgentReporting(AgentReportingChange{MetricsSec: 60, ReconnectSec: 10}); !errors.Is(err, ErrSettingsConflict) {
		t.Fatalf("stale reporting change = %v", err)
	}
}

func TestWebhookChangesApplyLiveAndKeepThePreviousSecretForADay(t *testing.T) {
	engine, _ := settingsEngine(t, legacyConfig(t))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	webhook, err := engine.WebhookSettings()
	if err != nil {
		t.Fatal(err)
	}
	webhook, err = engine.SaveWebhookSettings(WebhookChange{Revision: webhook.Revision, Enabled: true, Host: "127.0.0.1", Port: port, Path: "/hooks"})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port) + "/hooks"
	payload := `{"ref":"refs/heads/main","project":{"name":"api","git_http_url":"https://gitlab.com/acme/api.git"},"checkout_sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`
	sent := 0
	deliver := func(token string) int {
		sent++
		request, err := http.NewRequest("POST", endpoint, strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Gitlab-Token", token)
		request.Header.Set("X-Gitlab-Event", "Push Hook")
		request.Header.Set("X-Gitlab-Event-UUID", "delivery-"+strconv.Itoa(sent))
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	if status := deliver(legacyWebhookSecret); status != 202 {
		t.Fatalf("delivery on the new listener = %d", status)
	}
	if status := deliver("wrong-token"); status != 401 {
		t.Fatalf("wrong token = %d", status)
	}

	rotated, err := engine.RotateWebhookSecret(webhook.Revision)
	if err != nil || rotated.Secret == legacyWebhookSecret || rotated.Settings.PreviousUntil == nil {
		t.Fatalf("rotate = %+v %v", rotated.Settings, err)
	}
	if overlap := time.Until(*rotated.Settings.PreviousUntil); overlap > models.WebhookSecretOverlap || overlap < models.WebhookSecretOverlap-time.Minute {
		t.Fatalf("previous secret kept for %s", overlap)
	}
	for _, token := range []string{rotated.Secret, legacyWebhookSecret} {
		if status := deliver(token); status != 202 {
			t.Fatalf("token inside its overlap window = %d", status)
		}
	}
	deliveries, err := engine.WebhookDeliveries()
	if err != nil || len(deliveries) != 3 {
		t.Fatalf("deliveries = %+v %v", deliveries, err)
	}
	for _, delivery := range deliveries {
		if delivery.Result != models.DeliveryIgnored || delivery.Repository != "api" || delivery.Branch != "main" {
			t.Fatalf("delivery without a matching project = %+v", delivery)
		}
	}

	webhook, err = engine.SaveWebhookSettings(WebhookChange{Revision: rotated.Settings.Revision, Host: "127.0.0.1", Port: port, Path: "/hooks"})
	if err != nil {
		t.Fatal(err)
	}
	if status := deliver(rotated.Secret); status != 403 {
		t.Fatalf("delivery while disabled = %d", status)
	}
	if webhook, err = engine.WebhookSettings(); err != nil || webhook.Rejection == nil || webhook.Rejection.Reason != "deliveries are disabled" {
		t.Fatalf("the refused delivery is not reported: %+v %v", webhook.Rejection, err)
	}
}
