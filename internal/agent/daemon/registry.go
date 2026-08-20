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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const (
	dockerCertsDir = "/etc/docker/certs.d"
	loginTimeout   = 30 * time.Second
)

func (d *Daemon) applyRegistry(ctx context.Context, config ufp.RegistryConfig) error {
	d.registryMu.RLock()
	changed := d.registry != config
	d.registryMu.RUnlock()

	if !changed {
		return nil
	}
	if config.Host == "" || config.Username == "" || config.Password == "" || config.CACert == "" {
		return fmt.Errorf("registry configuration is incomplete")
	}

	if err := installCA(config); err != nil {
		return fmt.Errorf("install registry CA: %w", err)
	}

	if d.builder != nil {
		if err := login(ctx, config); err != nil {
			return fmt.Errorf("registry login: %w", err)
		}
	}

	d.registryMu.Lock()
	d.registry = config
	d.registryMu.Unlock()
	logger.Info("[AGENT] registry %s configured", config.Host)
	return nil
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
	path := filepath.Join(dir, "ca.crt")
	if current, err := os.ReadFile(path); err == nil && string(current) == config.CACert {
		return nil
	}
	return os.WriteFile(path, []byte(config.CACert), 0o644)
}

func login(parent context.Context, config ufp.RegistryConfig) error {
	ctx, cancel := context.WithTimeout(parent, loginTimeout)
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
