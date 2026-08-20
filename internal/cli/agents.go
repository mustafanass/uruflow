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
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/urustack/uruflow/internal/config"
	"github.com/urustack/uruflow/internal/models"
	"github.com/urustack/uruflow/internal/storage"
	"github.com/urustack/uruflow/internal/storage/sqlite"
	"github.com/urustack/uruflow/pkg/helper"
)

var agentRoles string

func agentCommands() *cobra.Command {
	group := &cobra.Command{
		Use:   "agent",
		Short: "Enrol and inspect agents without the terminal interface",
	}

	add := &cobra.Command{
		Use:   "add <name>",
		Short: "Enrol an agent and print its credentials",
		Args:  cobra.ExactArgs(1),
		RunE:  addAgent,
	}
	add.Flags().StringVar(&agentRoles, "roles", string(models.RoleRunner), "comma separated roles: builder, runner")

	group.AddCommand(
		add,
		&cobra.Command{Use: "list", Short: "List enrolled agents", RunE: listAgents},
		&cobra.Command{Use: "remove <name>", Short: "Remove an agent", Args: cobra.ExactArgs(1), RunE: removeAgent},
	)

	return group
}

func openStore() (*config.Config, storage.Store, error) {
	cfg, err := config.Load(configPath())
	if err != nil {
		return nil, nil, err
	}

	store, err := sqlite.New(cfg.DatabasePath())
	if err != nil {
		return nil, nil, err
	}
	return cfg, store, nil
}

func addAgent(_ *cobra.Command, args []string) error {
	cfg, store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	roles, err := parseRoles(agentRoles)
	if err != nil {
		return err
	}

	agent := &models.Agent{
		ID:    helper.GenerateID(),
		Name:  args[0],
		Key:   helper.GenerateToken(),
		Roles: roles,
	}
	if err := store.CreateAgent(agent); err != nil {
		return fmt.Errorf("an agent named %s already exists", agent.Name)
	}

	advertise := cfg.Server.Advertise
	if advertise == "" {
		advertise = "<server-host>"
	}

	fmt.Printf("enrolled %s\n\n", agent.Name)
	fmt.Printf("run this on the target machine:\n\n")
	fmt.Printf("  uruflow-agent init \\\n")
	fmt.Printf("    --id %s \\\n", agent.ID)
	fmt.Printf("    --key %s \\\n", agent.Key)
	fmt.Printf("    --server %s:%d \\\n", advertise, cfg.Server.UFPPort)
	fmt.Printf("    --roles %s\n\n", agentRoles)
	fmt.Printf("then copy the trust root:\n\n")
	fmt.Printf("  scp %s <host>:/etc/uruflow/ca.crt\n", cfg.CACertPath())
	return nil
}

func listAgents(*cobra.Command, []string) error {
	_, store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agents, err := store.ListAgents()
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		fmt.Println("no agents enrolled")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(writer, "NAME\tID\tROLES\tSTATUS\tHOST\tVERSION")
	for _, agent := range agents {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
			agent.Name, agent.ID, joinRoles(agent.Roles), agent.Status, agent.Host, agent.Version)
	}
	return writer.Flush()
}

func removeAgent(_ *cobra.Command, args []string) error {
	_, store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agent, err := store.GetAgentByName(args[0])
	if err != nil {
		return fmt.Errorf("no agent named %s", args[0])
	}
	if err := store.DeleteAgent(agent.ID); err != nil {
		return err
	}

	fmt.Printf("removed %s\n", agent.Name)
	return nil
}
