/*
 * Copyright (C) 2025 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of Uruflow, an open-source automation tool.
 *
 * Uruflow is a tool designed to streamline and automate Docker-based deployments.
 *
 * Uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3 of the License.
 *
 * Uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with Uruflow. If not, see <https://www.gnu.org/licenses/>.
 */

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"uruflow.com/env_manager"
	"uruflow.com/internal/models"
)

// Load and reads the configuration file using envManager
func Load(envManager *env_manager.EnvManager) (*models.Config, error) {
	configPath := filepath.Join(envManager.ConfigDir, "config.json")

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config models.Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	setDefaults(&config)
	return &config, nil
}

// GetConfigPath returns the configuration file path using envManager
func GetConfigPath(envManager *env_manager.EnvManager) string {
	return filepath.Join(envManager.ConfigDir, "config.json")
}

func WatchConfig(envManager *env_manager.EnvManager, callback func(*models.Config)) {
	configPath := filepath.Join(envManager.ConfigDir, "config.json")
	var lastModTime time.Time

	if fileInfo, err := os.Stat(configPath); err == nil {
		lastModTime = fileInfo.ModTime()
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			fileInfo, err := os.Stat(configPath)
			if err != nil {
				continue
			}

			if fileInfo.ModTime().After(lastModTime) {
				lastModTime = fileInfo.ModTime()

				if newConfig, err := Load(envManager); err == nil {
					callback(newConfig)
				}
			}
		}
	}()
}

// setDefaults applies default values to configuration using envManager
func setDefaults(config *models.Config) {
	if config.Settings.WorkDir == "" {
		config.Settings.WorkDir = "./repository"
	}
	if config.Settings.MaxConcurrent == 0 {
		config.Settings.MaxConcurrent = 3
	}
	if config.Webhook.Port == "" {
		config.Webhook.Port = "8080"
	}
	if config.Webhook.Path == "" {
		config.Webhook.Path = "/webhook"
	}
	for i := range config.Repositories {
		if config.Repositories[i].ComposeFile == "" {
			config.Repositories[i].ComposeFile = "docker-compose.yml"
		}
		config.Repositories[i].AutoDeploy = true
		config.Repositories[i].Enabled = true
	}
}
