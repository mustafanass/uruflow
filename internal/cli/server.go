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
	"syscall"
	"time"

	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/console"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/tui"
	"github.com/mustafanass/uruflow/internal/version"
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
	headless         bool
)

func ExecuteServer() error {
	root := &cobra.Command{
		Use:           "uruflow",
		Short:         "Self-hosted build and release infrastructure",
		Long:          "uruflow builds images on a builder agent, pushes them to its own registry, then releases those images to runner agents.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runDefault,
	}

	root.PersistentFlags().StringVarP(&serverConfigPath, "config", "c", "", "config file path")
	root.Flags().BoolVar(&headless, "headless", false, "run the server in the foreground (deprecated: use serve)")
	root.AddCommand(
		agentCommands(),
		&cobra.Command{
			Use:   "serve",
			Short: "Run the persistent server (for systemd)",
			RunE:  runServer,
		},
		&cobra.Command{
			Use:   "console",
			Short: "Attach the terminal dashboard to the running server",
			RunE:  runConsole,
		},
		&cobra.Command{
			Use:   "init",
			Short: "Create the server configuration",
			RunE:  runInit,
		},
		&cobra.Command{
			Use:   "version",
			Short: "Print the uruflow version",
			Run: func(*cobra.Command, []string) {
				fmt.Printf("uruflow %s\n", Version)
			},
		},
	)

	return root.Execute()
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

func runInit(*cobra.Command, []string) error {
	return tui.RunSetup(configPath())
}

func runDefault(cmd *cobra.Command, args []string) error {
	if headless {
		return runServer(cmd, args)
	}
	return runConsole(cmd, args)
}

func runConsole(*cobra.Command, []string) error {
	path := configPath()
	if !helper.Exists(path) {
		return fmt.Errorf("server config not found at %s; run uruflow init first", path)
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	return console.Attach(cfg.ConsoleSocketPath())
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

	if err := server.Start(); err != nil {
		return err
	}

	control, err := console.Listen(cfg.ConsoleSocketPath(), func(
		ctx context.Context, terminal *os.File, environment []string, sizes <-chan console.Size,
	) error {
		logger.Info("[CONSOLE] dashboard attached")
		defer logger.Info("[CONSOLE] dashboard detached")
		return tui.RunTerminal(ctx, server, terminal, environment, sizes)
	})
	if err != nil {
		shutdown(server)
		return err
	}
	defer control.Close()
	logger.Info("[CONSOLE] accepting local dashboards on %s", cfg.ConsoleSocketPath())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	logger.Info("[SERVER] running")
	<-signals
	control.Close()
	shutdown(server)
	return nil
}

func shutdown(server *api.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Info("[SERVER] shutting down")
	server.Shutdown(ctx)
}
