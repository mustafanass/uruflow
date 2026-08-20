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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/logic"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"
)

var ErrAgentOffline = errors.New("agent is not connected")

type Events interface {
	AgentConnected(agent *models.Agent)
	AgentDisconnected(agentID string)
	JobLog(agentID string, entry ufp.JobLog)
	JobStatus(agentID string, status ufp.JobStatus)
	ContainerLog(agentID string, entry ufp.ContainerLog)
}

type Server struct {
	cfg      *config.Config
	store    storage.Store
	registry ufp.RegistryConfig

	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc

	sessions map[string]*Session
	mu       sync.RWMutex

	subscribers []Events
	subMu       sync.RWMutex
}

func NewServer(cfg *config.Config, store storage.Store) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:      cfg,
		store:    store,
		ctx:      ctx,
		cancel:   cancel,
		sessions: make(map[string]*Session),
	}
}

func (s *Server) SetRegistry(registry ufp.RegistryConfig) {
	s.mu.Lock()
	s.registry = registry
	s.mu.Unlock()
}

func (s *Server) Subscribe(events Events) {
	s.subMu.Lock()
	s.subscribers = append(s.subscribers, events)
	s.subMu.Unlock()
}

func (s *Server) Start() error {
	tlsConfig, err := s.tlsConfig()
	if err != nil {
		return err
	}

	listener, err := tls.Listen("tcp", s.cfg.UFPAddr(), tlsConfig)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.cfg.UFPAddr(), err)
	}

	s.listener = listener
	logger.Info("[LINK] %s listening on %s", ufp.ProtocolName, s.cfg.UFPAddr())

	go s.accept()
	return nil
}

func (s *Server) Stop() error {
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	s.mu.Lock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	s.mu.Unlock()

	for _, session := range sessions {
		session.conn.SendGoodbye("server shutting down")
		session.conn.Close()
	}
	return nil
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return s.cfg.UFPAddr()
	}
	return s.listener.Addr().String()
}

func (s *Server) Online(agentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, found := s.sessions[agentID]
	return found
}

func (s *Server) OnlineAgents() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]string, 0, len(s.sessions))
	for agentID := range s.sessions {
		agents = append(agents, agentID)
	}
	return agents
}

func (s *Server) Request(ctx context.Context, agentID, method string, payload any) (*ufp.Response, error) {
	s.mu.RLock()
	session, found := s.sessions[agentID]
	s.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("%s: %w", agentID, ErrAgentOffline)
	}

	response, err := session.conn.Request(ctx, method, payload)
	if err != nil {
		return nil, err
	}
	if !response.OK {
		return nil, errors.New(response.ErrorMessage())
	}
	return response, nil
}

func (s *Server) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				logger.Error("[LINK] accept failed: %v", err)
				continue
			}
		}
		go s.handshake(conn)
	}
}

func (s *Server) handshake(netConn net.Conn) {
	conn, identity, err := ufp.Accept(netConn, s.lookupSecret)
	if err != nil {
		logger.Warn("[LINK] handshake with %s failed: %v", netConn.RemoteAddr(), err)
		netConn.Close()
		return
	}

	host, _, _ := net.SplitHostPort(netConn.RemoteAddr().String())
	session := newSession(s, conn, identity, host)

	s.register(session)
	defer s.unregister(session)

	if err := session.pushRegistry(); err != nil {
		logger.Warn("[LINK] registry handoff to %s failed: %v", identity.Name, err)
	}

	go conn.Heartbeat(s.ctx)

	logger.Info("[LINK] agent %s connected from %s roles=%v", identity.Name, host, identity.Roles)
	if err := conn.Serve(s.ctx, session); err != nil && !errors.Is(err, context.Canceled) {
		logger.Debug("[LINK] session with %s ended: %v", identity.Name, err)
	}
}

func (s *Server) lookupSecret(agentID string) (string, string, bool) {
	agent, err := s.store.GetAgent(agentID)
	if err != nil || agent == nil {
		return "", "", false
	}
	return agent.Key, agent.Name, true
}

func (s *Server) register(session *Session) {
	s.mu.Lock()
	if existing, found := s.sessions[session.identity.AgentID]; found {
		existing.conn.Close()
	}
	s.sessions[session.identity.AgentID] = session
	s.mu.Unlock()

	agent, err := s.store.GetAgent(session.identity.AgentID)
	if err != nil {
		return
	}

	agent.Host = session.host
	agent.Hostname = session.identity.Hostname
	agent.Version = session.identity.Version
	agent.Platform = session.identity.Platform
	agent.Roles = session.identity.Roles
	agent.Status = models.AgentOnline
	agent.LastSeen = time.Now()
	s.store.UpdateAgent(agent)

	s.notify(func(events Events) { events.AgentConnected(agent) })
}

func (s *Server) unregister(session *Session) {
	agentID := session.identity.AgentID

	s.mu.Lock()
	if current, found := s.sessions[agentID]; found && current == session {
		delete(s.sessions, agentID)
	}
	s.mu.Unlock()

	s.store.SetAgentStatus(agentID, models.AgentOffline)
	s.raiseAlert(logic.CheckAgentOffline(agentID, session.identity.Name))
	s.notify(func(events Events) { events.AgentDisconnected(agentID) })

	logger.Warn("[LINK] agent %s disconnected", session.identity.Name)
}

func (s *Server) notify(deliver func(Events)) {
	s.subMu.RLock()
	subscribers := make([]Events, len(s.subscribers))
	copy(subscribers, s.subscribers)
	s.subMu.RUnlock()

	for _, subscriber := range subscribers {
		deliver(subscriber)
	}
}

func (s *Server) tlsConfig() (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(s.cfg.ServerCertPath(), s.cfg.ServerKeyPath())
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
