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

package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/services"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/pkg/helper"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const webhookShutdown = 5 * time.Second

func (s *Server) Webhooks() *services.WebhookService { return s.webhook }

func (s *Server) WebhookSettings() (*models.Setting, models.WebhookSettings, error) {
	var settings models.WebhookSettings
	setting, err := readSetting(s.store, models.SettingWebhook, &settings)
	if errors.Is(err, storage.ErrNotFound) {
		if settings, err = s.importWebhookSettings(); err != nil {
			return nil, settings, err
		}
		setting, err = createSetting(s.store, models.SettingWebhook, settings)
		if errors.Is(err, storage.ErrConflict) {
			return s.WebhookSettings()
		}
	}
	return setting, settings, err
}

func (s *Server) ApplyWebhook(settings models.WebhookSettings) error {
	secret, err := s.vault.Open(settings.Secret)
	if err != nil {
		return fmt.Errorf("open webhook secret: %w", err)
	}
	previous := ""
	var previousUntil time.Time
	if settings.PreviousUntil != nil && time.Now().Before(*settings.PreviousUntil) {
		previous, err = s.vault.Open(settings.Previous)
		if err != nil {
			logger.Warn("[WEBHOOK] previous secret is unreadable and no longer accepted: %v", err)
		} else {
			previousUntil = *settings.PreviousUntil
		}
	}
	certificate, err := s.webhookCertificate(settings)
	if err != nil {
		return err
	}

	handler := s.routes(settings.Path)

	s.webhookMu.Lock()
	defer s.webhookMu.Unlock()
	routes := s.webhookRoutes.Load()
	current := s.webhookTLS.Load()
	s.webhookRoutes.Store(&handler)
	s.webhookTLS.Store(certificate)
	if err := s.serveWebhooks(s.webhookAddress(settings), settings.TLS); err != nil {
		if routes != nil {
			s.webhookRoutes.Store(routes)
		}
		s.webhookTLS.Store(current)
		return err
	}
	s.webhook.Apply(settings.Enabled, secret, previous, previousUntil)

	scheme := "https"
	if !settings.TLS {
		scheme = "http"
	}
	logger.Info("[HTTP] webhooks on %s://%s%s (enabled: %t)", scheme, s.webhookAddress(settings), settings.Path, settings.Enabled)
	return nil
}

func (s *Server) startWebhooks() error {
	_, settings, err := s.WebhookSettings()
	if err != nil {
		return err
	}
	if err := s.ApplyWebhook(settings); err != nil {
		logger.Error("[HTTP] webhooks are unavailable: %v; correct them in Settings → Webhooks", err)
	}
	return nil
}

func (s *Server) importWebhookSettings() (models.WebhookSettings, error) {
	legacy := s.cfg.Webhook
	settings := models.WebhookSettings{Enabled: true, Host: legacy.Host, Port: s.cfg.Server.HTTPPort, Path: legacy.Path, TLS: legacy.TLS == nil || *legacy.TLS}
	if settings.Path == "" {
		settings.Path = config.DefaultWebhookPath
	}
	if settings.Port == 0 {
		settings.Port = config.DefaultWebhookPort
	}
	secret := legacy.Secret
	if len(secret) < 32 {
		secret = helper.GenerateSecret()
	}
	sealed, err := s.vault.Seal(secret)
	if err != nil {
		return settings, err
	}
	settings.Secret, settings.SecretHint, settings.SecretAt = sealed, models.SecretHint(secret), time.Now().UTC()
	if legacy.Cert != "" {
		certificate, err := os.ReadFile(legacy.Cert)
		if err != nil {
			return settings, fmt.Errorf("import webhook certificate: %w", err)
		}
		key, err := os.ReadFile(legacy.Key)
		if err != nil {
			return settings, fmt.Errorf("import webhook key: %w", err)
		}
		settings.Certificate = string(certificate)
		if settings.CertificateKey, err = s.vault.Seal(string(key)); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

func (s *Server) webhookCertificate(settings models.WebhookSettings) (*tls.Certificate, error) {
	if !settings.TLS {
		return nil, nil
	}
	if settings.Certificate == "" {
		certificate, err := tls.LoadX509KeyPair(s.cfg.ServerCertPath(), s.cfg.ServerKeyPath())
		if err != nil {
			return nil, fmt.Errorf("load server certificate: %w", err)
		}
		return &certificate, nil
	}
	key, err := s.vault.Open(settings.CertificateKey)
	if err != nil {
		return nil, fmt.Errorf("open webhook certificate key: %w", err)
	}
	certificate, err := tls.X509KeyPair([]byte(settings.Certificate), []byte(key))
	if err != nil {
		return nil, fmt.Errorf("webhook certificate: %w", err)
	}
	return &certificate, nil
}

func (s *Server) webhookAddress(settings models.WebhookSettings) string {
	host := settings.Host
	if host == "" {
		host = s.cfg.Server.Host
	}
	return net.JoinHostPort(host, strconv.Itoa(settings.Port))
}

func (s *Server) serveWebhooks(address string, secure bool) error {
	if s.webhookServer != nil && s.webhookAddr == address && s.webhookSecure == secure {
		return nil
	}
	previous, previousAddress, previousSecure := s.webhookServer, s.webhookAddr, s.webhookSecure
	rebind := previous != nil && previousAddress == address
	if rebind {
		s.stopWebhooks(previous)
		s.webhookServer = nil
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		if rebind {
			if restoreErr := s.serveWebhooks(previousAddress, previousSecure); restoreErr != nil {
				logger.Error("[HTTP] restore webhooks on %s: %v", previousAddress, restoreErr)
			}
		}
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	if secure {
		listener = tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS13, GetCertificate: s.serveCertificate})
	}
	server := &http.Server{
		Handler:      http.HandlerFunc(s.serveWebhook),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("[HTTP] %v", err)
		}
	}()
	if previous != nil && !rebind {
		s.stopWebhooks(previous)
	}
	s.webhookServer, s.webhookAddr, s.webhookSecure = server, address, secure
	return nil
}

func (s *Server) stopWebhooks(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), webhookShutdown)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Warn("[HTTP] stop webhooks: %v", err)
	}
}

func (s *Server) serveWebhook(w http.ResponseWriter, r *http.Request) {
	(*s.webhookRoutes.Load()).ServeHTTP(w, r)
}

func (s *Server) serveCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certificate := s.webhookTLS.Load()
	if certificate == nil {
		return nil, errors.New("webhook certificate is not loaded")
	}
	return certificate, nil
}
