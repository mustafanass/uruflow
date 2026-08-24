//go:build unix

/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 */

package console

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

// The server console is intentionally local-only. Production uses this on
// Linux; the Unix build also keeps local development possible on macOS.

const (
	network          = "unix"
	handshakeTimeout = 5 * time.Second
	oobBuffer        = 1024
)

var terminalEnvironmentKeys = []string{
	"TERM",
	"COLORTERM",
	"NO_COLOR",
	"CLICOLOR",
	"CLICOLOR_FORCE",
	"TTY_FORCE",
	"GOOGLE_CLOUD_SHELL",
	"TMUX",
}

type environmentRequest struct {
	Environment []string `json:"environment"`
}

type Server struct {
	listener *net.UnixListener
	handler  Handler
	active   atomic.Bool
	closing  atomic.Bool

	mu    sync.Mutex
	conns map[*net.UnixConn]struct{}
	wg    sync.WaitGroup
}

func Listen(path string, handler Handler) (*Server, error) {
	if handler == nil {
		return nil, errors.New("console handler is required")
	}
	if err := prepareSocket(path); err != nil {
		return nil, err
	}

	address, err := net.ResolveUnixAddr(network, path)
	if err != nil {
		return nil, fmt.Errorf("resolve console socket: %w", err)
	}
	listener, err := net.ListenUnix(network, address)
	if err != nil {
		return nil, fmt.Errorf("listen on console socket %s: %w", path, err)
	}
	listener.SetUnlinkOnClose(true)
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, fmt.Errorf("secure console socket: %w", err)
	}

	server := &Server{
		listener: listener,
		handler:  handler,
		conns:    make(map[*net.UnixConn]struct{}),
	}
	server.wg.Add(1)
	go server.accept()
	return server, nil
}

func prepareSocket(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect console socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("console socket path %s is occupied by a non-socket file", path)
	}

	connection, dialErr := net.DialTimeout(network, path, 250*time.Millisecond)
	if dialErr == nil {
		connection.Close()
		return fmt.Errorf("another uruflow server is already listening on %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale console socket: %w", err)
	}
	return nil
}

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		connection, err := s.listener.AcceptUnix()
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

func (s *Server) handle(connection *net.UnixConn) {
	defer s.wg.Done()
	defer func() {
		connection.Close()
		s.mu.Lock()
		delete(s.conns, connection)
		s.mu.Unlock()
	}()

	terminal, err := receiveTerminal(connection)
	if err != nil {
		writeControlError(connection, err)
		return
	}
	defer terminal.Close()

	if !s.active.CompareAndSwap(false, true) {
		writeControlError(connection, errors.New("another console is already attached"))
		return
	}
	defer s.active.Store(false)
	if _, err := fmt.Fprintln(connection, "accepted"); err != nil {
		return
	}

	scanner := bufio.NewScanner(connection)
	environment, err := receiveEnvironment(connection, scanner)
	if err != nil {
		writeControlError(connection, err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sizes := make(chan Size, 1)
	go readControls(ctx, cancel, scanner, sizes)

	if _, err := fmt.Fprintln(connection, "ready"); err != nil {
		return
	}
	if err := s.handler(ctx, terminal, environment, sizes); err != nil {
		writeControlError(connection, err)
		return
	}
	fmt.Fprintln(connection, "done")
}

func receiveTerminal(connection *net.UnixConn) (*os.File, error) {
	if err := connection.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return nil, err
	}
	data := make([]byte, len("attach\n"))
	oob := make([]byte, oobBuffer)
	count, oobCount, _, _, err := connection.ReadMsgUnix(data, oob)
	connection.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, fmt.Errorf("receive console terminal: %w", err)
	}
	if strings.TrimSpace(string(data[:count])) != "attach" {
		return nil, errors.New("invalid console handshake")
	}

	messages, err := unix.ParseSocketControlMessage(oob[:oobCount])
	if err != nil {
		return nil, fmt.Errorf("parse console terminal: %w", err)
	}
	var descriptors []int
	for _, message := range messages {
		rights, rightsErr := unix.ParseUnixRights(&message)
		if rightsErr != nil {
			return nil, fmt.Errorf("parse console terminal rights: %w", rightsErr)
		}
		descriptors = append(descriptors, rights...)
	}
	if len(descriptors) != 1 {
		for _, descriptor := range descriptors {
			unix.Close(descriptor)
		}
		return nil, fmt.Errorf("console handshake supplied %d terminal descriptors, expected 1", len(descriptors))
	}
	return os.NewFile(uintptr(descriptors[0]), "uruflow-console-terminal"), nil
}

func receiveEnvironment(connection *net.UnixConn, scanner *bufio.Scanner) ([]string, error) {
	if err := connection.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return nil, err
	}
	if !scanner.Scan() {
		connection.SetReadDeadline(time.Time{})
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("receive terminal environment: %w", err)
		}
		return nil, errors.New("terminal environment was not supplied")
	}
	connection.SetReadDeadline(time.Time{})
	var request environmentRequest
	if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
		return nil, errors.New("invalid terminal environment")
	}
	return request.Environment, nil
}

func readControls(ctx context.Context, cancel context.CancelFunc, scanner *bufio.Scanner, sizes chan Size) {
	defer cancel()
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 || fields[0] != "resize" {
			continue
		}
		width, widthErr := strconv.Atoi(fields[1])
		height, heightErr := strconv.Atoi(fields[2])
		if widthErr != nil || heightErr != nil || width < 1 || height < 1 {
			continue
		}
		select {
		case sizes <- Size{Width: width, Height: height}:
		case <-ctx.Done():
			return
		default:
			select {
			case <-sizes:
			default:
			}
			select {
			case sizes <- Size{Width: width, Height: height}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func writeControlError(connection *net.UnixConn, err error) {
	fmt.Fprintf(connection, "error %s\n", strings.ReplaceAll(err.Error(), "\n", " "))
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

func Attach(path string) error {
	terminal, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	defer terminal.Close()
	return attachWithTerminal(path, terminal)
}

func attachWithTerminal(path string, terminal *os.File) error {
	terminalState, err := term.GetState(terminal.Fd())
	if err != nil {
		return fmt.Errorf("read terminal state: %w", err)
	}

	address, err := net.ResolveUnixAddr(network, path)
	if err != nil {
		return fmt.Errorf("resolve console socket: %w", err)
	}
	connection, err := net.DialUnix(network, nil, address)
	if err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) {
			return fmt.Errorf("connect to uruflow console: permission denied (run with sudo)")
		}
		return fmt.Errorf("uruflow server is not running (console socket %s): %w", path, err)
	}
	defer connection.Close()

	if err := sendTerminal(connection, terminal); err != nil {
		return fmt.Errorf("attach console terminal: %w", err)
	}
	reader := bufio.NewReader(connection)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("console handshake: %w", err)
	}
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "error ") {
		return errors.New(strings.TrimPrefix(response, "error "))
	}
	if response != "accepted" {
		return errors.New("invalid response from uruflow server")
	}
	defer func() {
		fmt.Fprint(terminal, "\x1b[?2004l\x1b[?25h\x1b[?1049l")
		_ = term.Restore(terminal.Fd(), terminalState)
	}()
	if err := sendEnvironment(connection, terminalEnvironment()); err != nil {
		return fmt.Errorf("send terminal environment: %w", err)
	}
	if err := sendSize(connection, terminal); err != nil {
		return err
	}

	resizes := make(chan os.Signal, 1)
	signal.Notify(resizes, syscall.SIGWINCH)
	defer signal.Stop(resizes)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-resizes:
				_ = sendSize(connection, terminal)
			case <-done:
				return
			}
		}
	}()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "ready":
			continue
		case line == "done":
			return nil
		case strings.HasPrefix(line, "error "):
			return errors.New(strings.TrimPrefix(line, "error "))
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("console connection: %w", err)
	}
	return errors.New("uruflow server closed the console connection")
}

func terminalEnvironment() []string {
	environment := make([]string, 0, len(terminalEnvironmentKeys))
	for _, key := range terminalEnvironmentKeys {
		if value, found := os.LookupEnv(key); found {
			environment = append(environment, key+"="+value)
		}
	}
	return environment
}

func sendTerminal(connection *net.UnixConn, terminal *os.File) error {
	rights := unix.UnixRights(int(terminal.Fd()))
	_, _, err := connection.WriteMsgUnix([]byte("attach\n"), rights, nil)
	return err
}

func sendEnvironment(connection *net.UnixConn, environment []string) error {
	payload, err := json.Marshal(environmentRequest{Environment: environment})
	if err != nil {
		return err
	}
	_, err = connection.Write(append(payload, '\n'))
	return err
}

func sendSize(connection *net.UnixConn, terminal *os.File) error {
	size, err := unix.IoctlGetWinsize(int(terminal.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return fmt.Errorf("read terminal size: %w", err)
	}
	if _, err := fmt.Fprintf(connection, "resize %d %d\n", size.Col, size.Row); err != nil {
		return fmt.Errorf("send terminal size: %w", err)
	}
	return nil
}
