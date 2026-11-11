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

package ufp

import "time"

const (
	ServerName   = "uruflow-server"
	AuthContext  = "uruflow-agent-auth-v1"
	ProtocolName = "UFP"
)

const (
	HandshakeTimeout = 10 * time.Second
	WriteTimeout     = 15 * time.Second
	PingInterval     = 20 * time.Second
	IdleTimeout      = 60 * time.Second
)

type Role string

const (
	RoleBuilder Role = "builder"
	RoleRunner  Role = "runner"
)

const (
	MethodBuildRun         = "build.run"
	MethodReleaseRun       = "release.run"
	MethodReleaseStop      = "release.stop"
	MethodReleaseRemove    = "release.remove"
	MethodLogsFollow       = "logs.follow"
	MethodLogsStop         = "logs.stop"
	MethodContainerRestart = "container.restart"
	MethodTerminalOpen     = "terminal.open"
	MethodTerminalInput    = "terminal.input"
	MethodTerminalResize   = "terminal.resize"
	MethodTerminalClose    = "terminal.close"
)

const (
	TopicRegistryConfig = "registry.config"
	TopicRegistryReady  = "registry.ready"
	TopicJobLog         = "job.log"
	TopicJobStatus      = "job.status"
	TopicMetrics        = "metrics.push"
	TopicContainerLog   = "container.log"
	TopicAgentConfig    = "agent.config"
	TopicTerminalOutput = "terminal.output"
)

const (
	StageBuild   = "build"
	StageRelease = "release"
)

const (
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

const (
	StreamStdout = "stdout"
	StreamStderr = "stderr"
)

const (
	AgentConfigSince     = "3.0.6"
	ResourceRemovalSince = "3.0.6"
	DefaultMetricsSec    = 10
	DefaultReconnectSec  = 5
	MinMetricsSec        = 2
	MaxMetricsSec        = 300
	MinReconnectSec      = 1
	MaxReconnectSec      = 60
)

const (
	MaxTerminalColumns = 500
	MaxTerminalRows    = 200
	MaxTerminalInput   = 64 * 1024
)

type LogFunc func(stream, line string)

func (r Role) Valid() bool {
	return r == RoleBuilder || r == RoleRunner
}

func ValidAgentConfig(config AgentConfig) bool {
	return config.MetricsSec >= MinMetricsSec && config.MetricsSec <= MaxMetricsSec &&
		config.ReconnectSec >= MinReconnectSec && config.ReconnectSec <= MaxReconnectSec
}

func ValidTerminalSize(columns, rows int) bool {
	return columns > 0 && columns <= MaxTerminalColumns && rows > 0 && rows <= MaxTerminalRows
}

func HasRole(roles []Role, want Role) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}
