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

package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/urustack/uruflow/internal/docker"
	"github.com/urustack/uruflow/internal/ufp"
	"github.com/urustack/uruflow/pkg/logger"
)

const (
	dockerCertsDir = "/etc/docker/certs.d"
	loginTimeout   = 30 * time.Second
)

func (d *Daemon) applyRegistry(config ufp.RegistryConfig) {
	d.registryMu.Lock()
	changed := d.registry != config
	d.registry = config
	d.registryMu.Unlock()

	if !changed {
		return
	}

	logger.Info("[AGENT] registry %s configured", config.Host)

	if err := installCA(config); err != nil {
		logger.Warn("[AGENT] could not install the registry CA: %v", err)
	}

	if d.builder != nil {
		if err := login(config); err != nil {
			logger.Warn("[AGENT] registry login failed: %v", err)
		}
	}
}

func (d *Daemon) registryAuth() *docker.Auth {
	d.registryMu.RLock()
	defer d.registryMu.RUnlock()

	if d.registry.Host == "" {
		return nil
	}
	return &docker.Auth{
		Username:      d.registry.Username,
		Password:      d.registry.Password,
		ServerAddress: d.registry.Host,
	}
}

func installCA(config ufp.RegistryConfig) error {
	if config.CACert == "" {
		return nil
	}

	dir := filepath.Join(dockerCertsDir, config.Host)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "ca.crt"), []byte(config.CACert), 0o644)
}

func login(config ufp.RegistryConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, "docker", "login", config.Host,
		"--username", config.Username, "--password-stdin")
	command.Stdin = strings.NewReader(config.Password)

	output, err := command.CombinedOutput()
	if err != nil {
		return err
	}

	logger.Debug("[AGENT] docker login: %s", output)
	return nil
}
