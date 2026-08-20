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

package link

import (
	"fmt"

	"github.com/mustafanass/uruflow/internal/ufp"
)

type Session struct {
	server   *Server
	conn     *ufp.Conn
	identity *ufp.Identity
	host     string
}

func newSession(server *Server, conn *ufp.Conn, identity *ufp.Identity, host string) *Session {
	return &Session{server: server, conn: conn, identity: identity, host: host}
}

func (s *Session) HandleRequest(request *ufp.Request) (any, error) {
	return nil, fmt.Errorf("ufp: agents may not call %q", request.Method)
}

func (s *Session) HandleEvent(event *ufp.Event) error {
	switch event.Topic {
	case ufp.TopicMetrics:
		var metrics ufp.Metrics
		if err := event.Decode(&metrics); err == nil {
			s.server.applyMetrics(s.identity, metrics)
		}

	case ufp.TopicJobLog:
		var entry ufp.JobLog
		if err := event.Decode(&entry); err == nil {
			s.server.notify(func(events Events) { events.JobLog(s.identity.AgentID, entry) })
		}

	case ufp.TopicJobStatus:
		var status ufp.JobStatus
		if err := event.Decode(&status); err == nil {
			s.server.notify(func(events Events) { events.JobStatus(s.identity.AgentID, status) })
		}

	case ufp.TopicContainerLog:
		var entry ufp.ContainerLog
		if err := event.Decode(&entry); err == nil {
			s.server.notify(func(events Events) { events.ContainerLog(s.identity.AgentID, entry) })
		}
	}

	return nil
}

func (s *Session) pushRegistry() error {
	s.server.mu.RLock()
	registry := s.server.registry
	s.server.mu.RUnlock()

	if registry.Host == "" {
		return nil
	}
	return s.conn.SendEvent(ufp.TopicRegistryConfig, registry)
}
