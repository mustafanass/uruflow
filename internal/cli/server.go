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

	"github.com/spf13/cobra"
	"github.com/urustack/uruflow/internal/api"
	"github.com/urustack/uruflow/internal/config"
	"github.com/urustack/uruflow/internal/storage/sqlite"
	"github.com/urustack/uruflow/internal/tui"
	"github.com/urustack/uruflow/pkg/helper"
	"github.com/urustack/uruflow/pkg/logger"
)

const (
	Version         = "2.0.0"
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
		RunE:          runServer,
	}

	root.PersistentFlags().StringVarP(&serverConfigPath, "config", "c", "", "config file path")
	root.Flags().BoolVar(&headless, "headless", false, "run without the terminal interface")
	root.AddCommand(
		agentCommands(),
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

func runServer(cmd *cobra.Command, args []string) error {
	path := configPath()

	if !helper.Exists(path) {
		if err := tui.RunSetup(path); err != nil {
			return err
		}
		if !helper.Exists(path) {
			return fmt.Errorf("setup did not complete")
		}
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
		if headless {
			logger.Init("", "info")
		} else {
			logger.Discard()
		}
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

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	if headless {
		logger.Info("[SERVER] running headless")
		<-signals
		shutdown(server)
		return nil
	}

	go func() {
		<-signals
		shutdown(server)
		os.Exit(0)
	}()

	if err := tui.Run(server); err != nil {
		shutdown(server)
		return err
	}

	shutdown(server)
	return nil
}

func shutdown(server *api.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Info("[SERVER] shutting down")
	server.Shutdown(ctx)
}
