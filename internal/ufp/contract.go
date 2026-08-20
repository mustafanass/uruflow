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
	MethodBuildRun      = "build.run"
	MethodReleaseRun    = "release.run"
	MethodReleaseStop   = "release.stop"
	MethodReleaseRemove = "release.remove"
	MethodLogsFollow    = "logs.follow"
	MethodLogsStop      = "logs.stop"
)

const (
	TopicRegistryConfig = "registry.config"
	TopicRegistryReady  = "registry.ready"
	TopicJobLog         = "job.log"
	TopicJobStatus      = "job.status"
	TopicMetrics        = "metrics.push"
	TopicContainerLog   = "container.log"
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

type LogFunc func(stream, line string)

func (r Role) Valid() bool {
	return r == RoleBuilder || r == RoleRunner
}

func HasRole(roles []Role, want Role) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}
