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
	"os"
	"strconv"
	"strings"

	"github.com/mustafanass/uruflow/internal/agent/config"
	"github.com/mustafanass/uruflow/internal/agent/daemon"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/spf13/cobra"
)

type enrollment struct {
	id     string
	key    string
	server string
	roles  string
	ca     string
	socket string
}

var (
	agentConfigPath string
	enrol           enrollment
)

func ExecuteAgent() error {
	root := &cobra.Command{
		Use:           "uruflow-agent",
		Short:         "uruflow build and release agent",
		Long:          "The agent builds images for uruflow, or pulls released images and runs them.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runAgent,
	}

	root.PersistentFlags().StringVarP(&agentConfigPath, "config", "c", "", "config file path")

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Write the agent configuration",
		RunE:  runAgentInit,
	}
	initCmd.Flags().StringVar(&enrol.id, "id", "", "agent id issued by the uruflow server")
	initCmd.Flags().StringVar(&enrol.key, "key", "", "agent key issued by the uruflow server")
	initCmd.Flags().StringVar(&enrol.server, "server", "", "uruflow server as host or host:port")
	initCmd.Flags().StringVar(&enrol.roles, "roles", string(ufp.RoleRunner), "comma separated roles: builder, runner")
	initCmd.Flags().StringVar(&enrol.ca, "ca", config.DefaultCACert, "path to the uruflow CA certificate")
	initCmd.Flags().StringVar(&enrol.socket, "docker-socket", "", "docker socket path")

	root.AddCommand(
		initCmd,
		&cobra.Command{Use: "run", Short: "Run the agent in the foreground", RunE: runAgent},
		&cobra.Command{Use: "stop", Short: "Stop a running agent", RunE: stopAgent},
		&cobra.Command{Use: "status", Short: "Show whether the agent is running", RunE: statusAgent},
		&cobra.Command{
			Use:   "version",
			Short: "Print the agent version",
			Run: func(*cobra.Command, []string) {
				fmt.Printf("uruflow-agent %s\n", config.Version)
			},
		},
	)

	return root.Execute()
}

func agentPath() string {
	if agentConfigPath != "" {
		return agentConfigPath
	}
	if fromEnv := os.Getenv("URUFLOW_AGENT_CONFIG"); fromEnv != "" {
		return fromEnv
	}
	return config.DefaultConfigPath
}

func runAgentInit(*cobra.Command, []string) error {
	if enrol.id == "" || enrol.key == "" || enrol.server == "" {
		return fmt.Errorf("--id, --key and --server are required; the uruflow agents view prints all three")
	}

	roles, err := parseRoles(enrol.roles)
	if err != nil {
		return err
	}

	host, port, err := splitServer(enrol.server)
	if err != nil {
		return err
	}

	cfg := config.Default()
	cfg.AgentID = enrol.id
	cfg.Key = enrol.key
	cfg.Roles = roles
	cfg.Server.Host = host
	cfg.Server.Port = port
	cfg.Server.CACert = enrol.ca
	if enrol.socket != "" {
		cfg.Docker.Socket = enrol.socket
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	path := agentPath()
	if err := cfg.Save(path); err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", path)
	fmt.Printf("roles: %s\n", enrol.roles)
	fmt.Printf("server: %s:%d\n", host, port)
	fmt.Printf("\ncopy the uruflow CA to %s, then run: uruflow-agent run\n", enrol.ca)
	return nil
}

func runAgent(*cobra.Command, []string) error {
	path := agentPath()

	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	agent, err := daemon.New(cfg)
	if err != nil {
		return err
	}
	return agent.Run()
}

func stopAgent(*cobra.Command, []string) error {
	cfg, err := config.Load(agentPath())
	if err != nil {
		return err
	}
	return daemon.Stop(cfg.PidFile)
}

func statusAgent(*cobra.Command, []string) error {
	cfg, err := config.Load(agentPath())
	if err != nil {
		return err
	}

	running, pid := daemon.IsRunning(cfg.PidFile)
	if !running {
		fmt.Println("uruflow-agent is not running")
		return nil
	}

	fmt.Printf("uruflow-agent is running (pid %d)\n", pid)
	fmt.Printf("server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("roles: %s\n", joinRoles(cfg.Roles))
	return nil
}

func parseRoles(value string) ([]ufp.Role, error) {
	roles := make([]ufp.Role, 0, 2)

	for _, part := range strings.Split(value, ",") {
		role := ufp.Role(strings.ToLower(strings.TrimSpace(part)))
		if role == "" {
			continue
		}
		if !role.Valid() {
			return nil, fmt.Errorf("unknown role %q: use builder, runner or both", part)
		}
		if !ufp.HasRole(roles, role) {
			roles = append(roles, role)
		}
	}

	if len(roles) == 0 {
		return nil, fmt.Errorf("at least one role is required: builder, runner")
	}
	return roles, nil
}

func joinRoles(roles []ufp.Role) string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, string(role))
	}
	return strings.Join(names, ", ")
}

func splitServer(value string) (string, int, error) {
	host, port, found := strings.Cut(value, ":")
	if !found {
		return value, config.Default().Server.Port, nil
	}

	number, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, fmt.Errorf("invalid server port %q", port)
	}
	return host, number, nil
}
