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

package control

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mustafanass/uruflow/internal/ops"
)

type Handler func(context.Context, []string, string, ops.Emit) error

type Server struct {
	listener net.Listener
	handler  Handler
	closing  atomic.Bool
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
}

func Listen(path string, handler Handler) (*Server, error) {
	if handler == nil {
		return nil, errors.New("control handler is required")
	}
	if err := prepareSocket(path); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on control socket %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, fmt.Errorf("secure control socket: %w", err)
	}
	server := &Server{listener: listener, handler: handler, conns: make(map[net.Conn]struct{})}
	server.wg.Add(1)
	go server.accept()
	return server, nil
}

func prepareSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect control socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("control socket path %s is occupied by a non-socket file", path)
	}
	connection, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond)
	if dialErr == nil {
		connection.Close()
		return fmt.Errorf("another uruflow server is already listening on %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale control socket: %w", err)
	}
	return nil
}

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			if s.closing.Load() {
				return
			}
			continue
		}
		s.mu.Lock()
		s.conns[connection] = struct{}{}
		s.mu.Unlock()
		s.wg.Add(1)
		go s.handle(connection)
	}
}

func (s *Server) handle(connection net.Conn) {
	defer s.wg.Done()
	defer func() {
		connection.Close()
		s.mu.Lock()
		delete(s.conns, connection)
		s.mu.Unlock()
	}()
	decoder := json.NewDecoder(bufio.NewReader(connection))
	encoder := json.NewEncoder(connection)
	var request Request
	if err := decoder.Decode(&request); err != nil {
		_ = encoder.Encode(Response{Error: "invalid control request"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, _ = io.Copy(io.Discard, connection)
		cancel()
	}()
	emit := func(event ops.Event) error {
		if event.Time.IsZero() {
			event.Time = time.Now()
		}
		return encoder.Encode(Response{Event: &event})
	}
	if err := s.handler(ctx, request.Args, request.Input, emit); err != nil {
		_ = encoder.Encode(Response{Error: err.Error()})
		return
	}
	_ = encoder.Encode(Response{Done: true})
}

func (s *Server) Close() error {
	if !s.closing.CompareAndSwap(false, true) {
		return nil
	}
	err := s.listener.Close()
	s.mu.Lock()
	for connection := range s.conns {
		connection.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return err
}
