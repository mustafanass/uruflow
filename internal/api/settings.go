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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/secrets"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/helper"
	"github.com/mustafanass/uruflow/pkg/logger"
)

func readSetting(store storage.Store, name string, value any) (*models.Setting, error) {
	setting, err := store.GetSetting(name)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(setting.Value), value); err != nil {
		return nil, fmt.Errorf("read %s settings: %w", name, err)
	}
	return setting, nil
}

func createSetting(store storage.Store, name string, value any) (*models.Setting, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	setting := &models.Setting{Name: name, Value: string(encoded)}
	if err := store.SaveSetting(setting); err != nil {
		return nil, err
	}
	logger.Info("[SETTINGS] %s settings moved to the database; manage them in Settings", name)
	return setting, nil
}

func (s *Server) RegistrySettings() (*models.Setting, models.RegistrySettings, error) {
	return loadRegistrySettings(s.cfg, s.store, s.vault)
}

func loadRegistrySettings(cfg *config.Config, store storage.Store, vault *secrets.Vault) (*models.Setting, models.RegistrySettings, error) {
	var settings models.RegistrySettings
	setting, err := readSetting(store, models.SettingRegistry, &settings)
	if errors.Is(err, storage.ErrNotFound) {
		if settings, err = importRegistrySettings(cfg, vault); err != nil {
			return nil, settings, err
		}
		setting, err = createSetting(store, models.SettingRegistry, settings)
		if errors.Is(err, storage.ErrConflict) {
			return loadRegistrySettings(cfg, store, vault)
		}
	}
	return setting, settings, err
}

func importRegistrySettings(cfg *config.Config, vault *secrets.Vault) (models.RegistrySettings, error) {
	settings := models.RegistrySettings{RetainReleases: models.DefaultRetainReleases, PasswordAt: time.Now().UTC()}
	if cfg.Registry.RetainReleases != nil {
		settings.RetainReleases = *cfg.Registry.RetainReleases
	}
	password := cfg.Registry.Password
	if len(password) < 16 {
		password = helper.GenerateToken()
	}
	sealed, err := vault.Seal(password)
	if err != nil {
		return settings, err
	}
	settings.Password = sealed
	return settings, nil
}

func (s *Server) warnImportedConfig() {
	webhook := s.cfg.Webhook != (config.WebhookConfig{}) || s.cfg.Server.HTTPPort != 0
	registry := s.cfg.Registry.Password != "" || s.cfg.Registry.RetainReleases != nil
	if webhook || registry {
		logger.Warn("[CONFIG] webhook, server.http_port, registry.password and registry.retain_releases in config.yaml are no longer read; remove them")
	}
}

func (s *Server) ApplyRegistrySettings(settings models.RegistrySettings) {
	s.registryRetain.Store(int64(settings.RetainReleases))
}

func (s *Server) RotateRegistryPassword(ctx context.Context, password string) error {
	if err := s.registry.SetPassword(ctx, password); err != nil {
		return err
	}
	s.link.UpdateRegistry(ufp.RegistryConfig{
		Host:     s.cfg.Registry.Address(),
		Username: s.cfg.Registry.Username,
		Password: password,
		CACert:   s.caCert,
	})
	return nil
}

func (s *Server) AgentSettings() (*models.Setting, models.AgentSettings, error) {
	var settings models.AgentSettings
	setting, err := readSetting(s.store, models.SettingAgents, &settings)
	return setting, settings, err
}

func (s *Server) ApplyAgentSettings(settings models.AgentSettings) {
	s.link.UpdateAgentConfig(ufp.AgentConfig{MetricsSec: settings.MetricsSec, ReconnectSec: settings.ReconnectSec})
}

func (s *Server) retainReleases() int {
	return int(s.registryRetain.Load())
}
