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
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/ops"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/version"
	"github.com/mustafanass/uruflow/internal/web"
	"github.com/mustafanass/uruflow/pkg/helper"
	"github.com/mustafanass/uruflow/pkg/logger"
	"github.com/spf13/cobra"
)

const (
	Version         = version.Current
	shutdownTimeout = 15 * time.Second
)

var (
	serverConfigPath string
)

func ExecuteServer() error {
	return newServerCommand().Execute()
}

func newServerCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "uruflow",
		Short:         "Run the URUFLOW deployment control plane and Web UI",
		Long:          "URUFLOW serves its browser operator interface and deployment API. Agents connect over the TLS UFP link.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          func(command *cobra.Command, _ []string) error { return command.Help() },
	}

	root.PersistentFlags().StringVarP(&serverConfigPath, "config", "c", "", "config file path")
	root.AddCommand(
		&cobra.Command{
			Use:   "serve",
			Short: "Run the persistent server (for systemd)",
			Args:  cobra.NoArgs,
			RunE:  runServer,
		},
		initCommand(),
		adminCommand(),
		&cobra.Command{
			Use:   "version",
			Short: "Print the uruflow version",
			Run: func(*cobra.Command, []string) {
				fmt.Printf("uruflow %s\n", Version)
			},
		},
	)

	return root
}

func configPath() string {
	if serverConfigPath != "" {
		return serverConfigPath
	}
	if fromEnv := os.Getenv("URUFLOW_CONFIG"); fromEnv != "" {
		return fromEnv
	}
	return config.DefaultConfigPath
}

func runServer(*cobra.Command, []string) error {
	path := configPath()

	if !helper.Exists(path) {
		return fmt.Errorf("server config not found at %s; run uruflow init first", path)
	}

	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if err := cfg.EnsureDirs(); err != nil {
		return err
	}

	if err := logger.Init(cfg.LogPath(), "info"); err != nil {
		logger.Init("", "info")
	}
	logger.Info("[SERVER] uruflow %s starting", Version)

	store, err := sqlite.New(cfg.DatabasePath())
	if err != nil {
		return err
	}
	defer store.Close()

	server, err := api.NewServer(cfg, store)
	if err != nil {
		return err
	}

	if !web.AssetsReady() {
		return fmt.Errorf("frontend assets are missing; build URUFLOW with make build")
	}
	operations := ops.New(server)
	console, err := web.New(operations, store, cfg.Server.DataDir, cfg.Web.TLS || strings.HasPrefix(cfg.Web.PublicURL, "https://"))
	if err != nil {
		return fmt.Errorf("operator interface: %w", err)
	}
	defer console.Close()
	if err := server.Start(); err != nil {
		return err
	}
	if err := console.Listen(cfg); err != nil {
		shutdown(server)
		return fmt.Errorf("operator listener: %w", err)
	}
	logger.Info("[WEB] browser operator interface on %s; first administrator setup token: %s", cfg.WebAddr(), console.SetupTokenPath())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	logger.Info("[SERVER] running")
	<-signals
	console.Close()
	shutdown(server)
	return nil
}

func shutdown(server *api.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Info("[SERVER] shutting down")
	server.Shutdown(ctx)
}
