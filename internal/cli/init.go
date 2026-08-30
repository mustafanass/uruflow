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

package cli

import (
	"fmt"
	"strings"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/spf13/cobra"
)

var (
	initAdvertise    string
	initRegistryHost string
	initDataDir      string
)

func initCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "init",
		Short: "Create the YAML server configuration",
		RunE:  runInit,
	}
	command.Flags().StringVar(&initAdvertise, "advertise", "", "hostname or IP that agents dial (required)")
	command.Flags().StringVar(&initRegistryHost, "registry-host", "", "registry hostname reachable by agents (defaults to --advertise)")
	command.Flags().StringVar(&initDataDir, "data-dir", "", "state directory (defaults to /var/lib/uruflow)")
	return command
}

func runInit(*cobra.Command, []string) error {
	advertise := strings.TrimSpace(initAdvertise)
	if advertise == "" {
		return fmt.Errorf("--advertise is required; for example: uruflow init --advertise uruflow.internal")
	}
	cfg := config.Default()
	cfg.Server.Advertise = advertise
	cfg.Registry.Host = strings.TrimSpace(initRegistryHost)
	if cfg.Registry.Host == "" {
		cfg.Registry.Host = advertise
	}
	if initDataDir != "" {
		cfg.Server.DataDir = initDataDir
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := cfg.Save(configPath()); err != nil {
		return err
	}
	fmt.Printf("created %s\n", configPath())
	fmt.Printf("projects: %s\n", cfg.ProjectsDir())
	fmt.Println("next: start uruflow serve, then enrol a builder and runner")
	return nil
}
