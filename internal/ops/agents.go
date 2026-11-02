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

package ops

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/roles"
	"github.com/mustafanass/uruflow/internal/services"
	"github.com/mustafanass/uruflow/pkg/helper"
)

type Enrollment struct {
	Agent     models.Agent `json:"agent"`
	Command   string       `json:"command"`
	TrustRoot string       `json:"trust_root"`
}

func (e *Engine) EnrollAgent(name string, roleNames []string) (Enrollment, error) {
	var result Enrollment
	if !models.ValidResourceName(name) {
		return result, fmt.Errorf("invalid agent name")
	}
	parsed, err := roles.Parse(strings.Join(roleNames, ","))
	if err != nil {
		return result, err
	}
	agent := models.Agent{ID: helper.GenerateID(), Name: name, Key: helper.GenerateToken(), Roles: parsed, Status: models.AgentOffline}
	if err = e.server.Store().CreateAgent(&agent); err != nil {
		return result, fmt.Errorf("enroll agent: %w", err)
	}
	cfg := e.server.Config()
	address := net.JoinHostPort(cfg.Server.Advertise, strconv.Itoa(cfg.Server.UFPPort))
	result.Agent = agent
	result.Command = "sudo uruflow-agent init --id " + shellQuote(agent.ID) + " --key " + shellQuote(agent.Key) + " --server " + shellQuote(address) + " --roles " + shellQuote(strings.Join(roleNames, ","))
	result.TrustRoot = "/api/trust-root"
	return result, nil
}
func (e *Engine) RemoveAgent(name string) error {
	agent, err := e.server.Store().GetAgentByName(name)
	if err != nil {
		return err
	}
	return services.DeleteAgent(e.server.Store(), e.server.Link().Revoke, agent.ID)
}
func (e *Engine) TrustRoot() ([]byte, error) { return os.ReadFile(e.server.Config().CACertPath()) }
func shellQuote(value string) string         { return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'" }
