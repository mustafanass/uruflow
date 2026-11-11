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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/services"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/helper"
)

const (
	maxCertificateSize      = 64 << 10
	deliveryLimit           = 100
	registryRotationTimeout = 2 * time.Minute
)

var ErrSettingsConflict = errors.New("these settings changed since you opened them; reload before saving")

type WebhookView struct {
	Revision      int64               `json:"revision"`
	Enabled       bool                `json:"enabled"`
	Endpoint      string              `json:"endpoint"`
	Host          string              `json:"host"`
	Port          int                 `json:"port"`
	Path          string              `json:"path"`
	TLS           bool                `json:"tls"`
	Certificate   *CertificateView    `json:"certificate,omitempty"`
	SecretHint    string              `json:"secret_hint"`
	SecretAt      time.Time           `json:"secret_at"`
	PreviousUntil *time.Time          `json:"previous_until,omitempty"`
	Rejection     *services.Rejection `json:"rejection,omitempty"`
}

type CertificateView struct {
	Subject   string    `json:"subject"`
	Names     []string  `json:"names"`
	ExpiresAt time.Time `json:"expires_at"`
}

type WebhookChange struct {
	Revision int64  `json:"revision"`
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Path     string `json:"path"`
	TLS      bool   `json:"tls"`
}

type WebhookSecret struct {
	Secret   string      `json:"secret"`
	Settings WebhookView `json:"settings"`
}

func (e *Engine) WebhookSettings() (WebhookView, error) {
	setting, settings, err := e.server.WebhookSettings()
	if err != nil {
		return WebhookView{}, err
	}
	return e.webhookView(setting.Revision, settings), nil
}

func (e *Engine) SaveWebhookSettings(change WebhookChange) (WebhookView, error) {
	return e.updateWebhook(change.Revision, func(settings *models.WebhookSettings) error {
		settings.Enabled = change.Enabled
		settings.Host = strings.TrimSpace(change.Host)
		settings.Port = change.Port
		settings.Path = change.Path
		settings.TLS = change.TLS
		return nil
	})
}

func (e *Engine) RotateWebhookSecret(revision int64) (WebhookSecret, error) {
	secret := helper.GenerateSecret()
	view, err := e.updateWebhook(revision, func(settings *models.WebhookSettings) error {
		sealed, err := e.server.Vault().Seal(secret)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		until := now.Add(models.WebhookSecretOverlap)
		settings.Previous, settings.PreviousUntil = settings.Secret, &until
		settings.Secret, settings.SecretHint, settings.SecretAt = sealed, models.SecretHint(secret), now
		return nil
	})
	if err != nil {
		return WebhookSecret{}, err
	}
	return WebhookSecret{Secret: secret, Settings: view}, nil
}

func (e *Engine) SetWebhookCertificate(revision int64, certificate, key string) (WebhookView, error) {
	return e.updateWebhook(revision, func(settings *models.WebhookSettings) error {
		if certificate == "" && key == "" {
			settings.Certificate, settings.CertificateKey = "", nil
			return nil
		}
		if len(certificate) > maxCertificateSize || len(key) > maxCertificateSize {
			return errors.New("certificate or key is too large")
		}
		if _, err := tls.X509KeyPair([]byte(certificate), []byte(key)); err != nil {
			return fmt.Errorf("certificate and key do not form a valid pair: %w", err)
		}
		sealed, err := e.server.Vault().Seal(key)
		if err != nil {
			return err
		}
		settings.Certificate, settings.CertificateKey = certificate, sealed
		return nil
	})
}

func (e *Engine) WebhookDeliveries() ([]models.WebhookDelivery, error) {
	return e.server.Store().ListWebhookDeliveries(deliveryLimit)
}

func (e *Engine) updateWebhook(revision int64, change func(*models.WebhookSettings) error) (WebhookView, error) {
	e.settingsMu.Lock()
	defer e.settingsMu.Unlock()
	setting, current, err := e.server.WebhookSettings()
	if err != nil {
		return WebhookView{}, err
	}
	if setting.Revision != revision {
		return WebhookView{}, ErrSettingsConflict
	}
	next := current
	if err := change(&next); err != nil {
		return WebhookView{}, err
	}
	if next.PreviousUntil != nil && !time.Now().Before(*next.PreviousUntil) {
		next.Previous, next.PreviousUntil = nil, nil
	}
	if err := e.validateWebhook(next); err != nil {
		return WebhookView{}, err
	}
	if err := e.server.ApplyWebhook(next); err != nil {
		return WebhookView{}, err
	}
	if err := e.saveSetting(setting, next); err != nil {
		if restoreErr := e.server.ApplyWebhook(current); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore webhooks: %w", restoreErr))
		}
		return WebhookView{}, err
	}
	return e.webhookView(setting.Revision, next), nil
}

func (e *Engine) saveSetting(setting *models.Setting, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	setting.Value = string(encoded)
	if err := e.server.Store().SaveSetting(setting); errors.Is(err, storage.ErrConflict) {
		return ErrSettingsConflict
	} else if err != nil {
		return err
	}
	return nil
}

func (e *Engine) validateWebhook(settings models.WebhookSettings) error {
	cfg := e.server.Config()
	if settings.Port < 1 || settings.Port > 65535 {
		return errors.New("webhook port is outside the valid range")
	}
	if settings.Port == cfg.Web.Port || settings.Port == cfg.Server.UFPPort || settings.Port == cfg.Registry.Port {
		return errors.New("webhook port must be separate from the web, UFP and registry ports")
	}
	if !strings.HasPrefix(settings.Path, "/") || strings.ContainsAny(settings.Path, ":*{}?#\\ \t\r\n") {
		return errors.New("webhook path must be a literal absolute URL path")
	}
	if settings.Path == "/health" || settings.Path == "/api" || strings.HasPrefix(settings.Path, "/api/") {
		return errors.New("webhook path cannot use /health or the operator API namespace")
	}
	host := strings.Trim(settings.Host, "[]")
	if host != "" && host != "localhost" && net.ParseIP(host) == nil {
		return errors.New("webhook host must be an IP address or localhost")
	}
	if host == "" {
		host = strings.Trim(cfg.Server.Host, "[]")
	}
	if !settings.TLS {
		address := net.ParseIP(host)
		if host != "localhost" && (address == nil || !address.IsLoopback()) {
			return errors.New("plain HTTP webhooks require a loopback host behind an HTTPS proxy")
		}
		if settings.Certificate != "" {
			return errors.New("remove the custom certificate before turning TLS off")
		}
	}
	if len(settings.Secret) == 0 {
		return errors.New("webhook secret is missing; regenerate it")
	}
	return nil
}

func (e *Engine) webhookView(revision int64, settings models.WebhookSettings) WebhookView {
	view := WebhookView{
		Revision:   revision,
		Enabled:    settings.Enabled,
		Endpoint:   settings.URL(e.server.Config().Server.Advertise),
		Host:       settings.Host,
		Port:       settings.Port,
		Path:       settings.Path,
		TLS:        settings.TLS,
		SecretHint: settings.SecretHint,
		SecretAt:   settings.SecretAt,
	}
	if settings.PreviousUntil != nil && time.Now().Before(*settings.PreviousUntil) {
		view.PreviousUntil = settings.PreviousUntil
	}
	if settings.Certificate != "" {
		view.Certificate = certificateView(settings.Certificate)
	}
	if rejection, ok := e.server.Webhooks().LastRejection(); ok {
		view.Rejection = &rejection
	}
	return view
}

func certificateView(certificate string) *CertificateView {
	view := &CertificateView{Names: []string{}}
	block, _ := pem.Decode([]byte(certificate))
	if block == nil {
		return view
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return view
	}
	view.Subject, view.ExpiresAt = leaf.Subject.CommonName, leaf.NotAfter
	view.Names = append(view.Names, leaf.DNSNames...)
	return view
}

type RegistryView struct {
	Revision       int64     `json:"revision"`
	RetainReleases int       `json:"retain_releases"`
	PasswordAt     time.Time `json:"password_at"`
}

type RegistryChange struct {
	Revision       int64 `json:"revision"`
	RetainReleases int   `json:"retain_releases"`
}

func (e *Engine) RegistrySettings() (RegistryView, error) {
	setting, settings, err := e.server.RegistrySettings()
	if err != nil {
		return RegistryView{}, err
	}
	return registryView(setting.Revision, settings), nil
}

func (e *Engine) SaveRegistrySettings(change RegistryChange) (RegistryView, error) {
	e.settingsMu.Lock()
	defer e.settingsMu.Unlock()
	setting, settings, err := e.server.RegistrySettings()
	if err != nil {
		return RegistryView{}, err
	}
	if setting.Revision != change.Revision {
		return RegistryView{}, ErrSettingsConflict
	}
	if change.RetainReleases < 0 || change.RetainReleases > models.MaxRetainReleases {
		return RegistryView{}, fmt.Errorf("keep between 0 and %d images per service", models.MaxRetainReleases)
	}
	settings.RetainReleases = change.RetainReleases
	if err := e.saveSetting(setting, settings); err != nil {
		return RegistryView{}, err
	}
	e.server.ApplyRegistrySettings(settings)
	return registryView(setting.Revision, settings), nil
}

func (e *Engine) RotateRegistryPassword(revision int64) (RegistryView, error) {
	e.settingsMu.Lock()
	defer e.settingsMu.Unlock()
	setting, settings, err := e.server.RegistrySettings()
	if err != nil {
		return RegistryView{}, err
	}
	if setting.Revision != revision {
		return RegistryView{}, ErrSettingsConflict
	}
	previous, err := e.server.Vault().Open(settings.Password)
	if err != nil {
		return RegistryView{}, fmt.Errorf("open registry password: %w", err)
	}
	password := helper.GenerateToken()
	sealed, err := e.server.Vault().Seal(password)
	if err != nil {
		return RegistryView{}, err
	}
	err = e.server.Pipeline().RegistryExclusive(func() error {
		if err := e.rotateRegistry(password); err != nil {
			return errors.Join(err, e.restoreRegistry(previous))
		}
		settings.Password, settings.PasswordAt = sealed, time.Now().UTC()
		if err := e.saveSetting(setting, settings); err != nil {
			return errors.Join(err, e.restoreRegistry(previous))
		}
		return nil
	})
	if err != nil {
		return RegistryView{}, err
	}
	return registryView(setting.Revision, settings), nil
}

func (e *Engine) rotateRegistry(password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), registryRotationTimeout)
	defer cancel()
	return e.server.RotateRegistryPassword(ctx, password)
}

func (e *Engine) restoreRegistry(password string) error {
	if err := e.rotateRegistry(password); err != nil {
		return fmt.Errorf("restore registry password: %w", err)
	}
	return nil
}

func registryView(revision int64, settings models.RegistrySettings) RegistryView {
	return RegistryView{Revision: revision, RetainReleases: settings.RetainReleases, PasswordAt: settings.PasswordAt}
}

type AgentReportingView struct {
	Revision     int64  `json:"revision"`
	Managed      bool   `json:"managed"`
	MetricsSec   int    `json:"metrics_sec"`
	ReconnectSec int    `json:"reconnect_sec"`
	Since        string `json:"since"`
}

type AgentReportingChange struct {
	Revision     int64 `json:"revision"`
	MetricsSec   int   `json:"metrics_sec"`
	ReconnectSec int   `json:"reconnect_sec"`
}

func (e *Engine) AgentReporting() (AgentReportingView, error) {
	setting, settings, err := e.server.AgentSettings()
	if errors.Is(err, storage.ErrNotFound) {
		return AgentReportingView{MetricsSec: ufp.DefaultMetricsSec, ReconnectSec: ufp.DefaultReconnectSec, Since: ufp.AgentConfigSince}, nil
	}
	if err != nil {
		return AgentReportingView{}, err
	}
	return AgentReportingView{Revision: setting.Revision, Managed: true, MetricsSec: settings.MetricsSec, ReconnectSec: settings.ReconnectSec, Since: ufp.AgentConfigSince}, nil
}

func (e *Engine) SaveAgentReporting(change AgentReportingChange) (AgentReportingView, error) {
	e.settingsMu.Lock()
	defer e.settingsMu.Unlock()
	setting, _, err := e.server.AgentSettings()
	if errors.Is(err, storage.ErrNotFound) {
		setting = &models.Setting{Name: models.SettingAgents}
	} else if err != nil {
		return AgentReportingView{}, err
	}
	if setting.Revision != change.Revision {
		return AgentReportingView{}, ErrSettingsConflict
	}
	settings := models.AgentSettings{MetricsSec: change.MetricsSec, ReconnectSec: change.ReconnectSec}
	if !ufp.ValidAgentConfig(ufp.AgentConfig{MetricsSec: settings.MetricsSec, ReconnectSec: settings.ReconnectSec}) {
		return AgentReportingView{}, fmt.Errorf("report every %d–%d seconds and reconnect after %d–%d seconds",
			ufp.MinMetricsSec, ufp.MaxMetricsSec, ufp.MinReconnectSec, ufp.MaxReconnectSec)
	}
	if err := e.saveSetting(setting, settings); err != nil {
		return AgentReportingView{}, err
	}
	e.server.ApplyAgentSettings(settings)
	return AgentReportingView{Revision: setting.Revision, Managed: true, MetricsSec: settings.MetricsSec, ReconnectSec: settings.ReconnectSec, Since: ufp.AgentConfigSince}, nil
}
