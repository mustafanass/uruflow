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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServerReceivesTerminalAndResize(t *testing.T) {
	path := socketPath(t)
	received := make(chan Size, 1)
	server, err := Listen(path, func(ctx context.Context, terminal *os.File, environment []string, sizes <-chan Size) error {
		if _, err := terminal.Stat(); err != nil {
			return fmt.Errorf("terminal descriptor: %w", err)
		}
		if len(environment) != 1 || environment[0] != "TERM=xterm-256color" {
			return fmt.Errorf("terminal environment: %v", environment)
		}
		select {
		case size := <-sizes:
			received <- size
			return nil
		case <-time.After(time.Second):
			return fmt.Errorf("resize was not delivered")
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	address, err := net.ResolveUnixAddr(network, path)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	connection, err := net.DialUnix(network, nil, address)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer connection.Close()

	terminal, peer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer terminal.Close()
	defer peer.Close()

	if err := sendTerminal(connection, terminal); err != nil {
		t.Fatalf("send terminal: %v", err)
	}
	reader := bufio.NewReader(connection)
	if response, err := reader.ReadString('\n'); err != nil || response != "accepted\n" {
		t.Fatalf("accepted response = %q, err = %v", response, err)
	}
	if err := sendEnvironment(connection, []string{"TERM=xterm-256color"}); err != nil {
		t.Fatalf("send environment: %v", err)
	}
	if _, err := fmt.Fprintln(connection, "resize 132 41"); err != nil {
		t.Fatalf("send resize: %v", err)
	}

	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatalf("ready response = %q, err = %v", scanner.Text(), scanner.Err())
	}
	if !scanner.Scan() || scanner.Text() != "done" {
		t.Fatalf("done response = %q, err = %v", scanner.Text(), scanner.Err())
	}

	select {
	case size := <-received:
		if size.Width != 132 || size.Height != 41 {
			t.Fatalf("size = %+v", size)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not receive resize")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("socket permissions = %o", info.Mode().Perm())
	}
}

func TestOnlyOneConsoleMayAttach(t *testing.T) {
	path := socketPath(t)
	release := make(chan struct{})
	started := make(chan struct{})
	server, err := Listen(path, func(context.Context, *os.File, []string, <-chan Size) error {
		close(started)
		<-release
		return nil
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		server.Close()
	})

	first, firstReader := dialWithDescriptor(t, path)
	defer first.Close()
	if response, err := firstReader.ReadString('\n'); err != nil || response != "accepted\n" {
		t.Fatalf("first accepted response = %q, err = %v", response, err)
	}
	if err := sendEnvironment(first, []string{"TERM=xterm"}); err != nil {
		t.Fatalf("send first environment: %v", err)
	}
	firstScanner := bufio.NewScanner(firstReader)
	if !firstScanner.Scan() || firstScanner.Text() != "ready" {
		t.Fatalf("first response = %q", firstScanner.Text())
	}
	<-started

	second, secondReader := dialWithDescriptor(t, path)
	defer second.Close()
	response, err := secondReader.ReadString('\n')
	if err != nil || response != "error another console is already attached\n" {
		t.Fatalf("second response = %q, err = %v", response, err)
	}
	close(release)
}

func dialWithDescriptor(t *testing.T, path string) (*net.UnixConn, *bufio.Reader) {
	t.Helper()
	address, err := net.ResolveUnixAddr(network, path)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	connection, err := net.DialUnix(network, nil, address)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	terminal, peer, err := os.Pipe()
	if err != nil {
		connection.Close()
		t.Fatalf("pipe: %v", err)
	}
	if err := sendTerminal(connection, terminal); err != nil {
		connection.Close()
		terminal.Close()
		peer.Close()
		t.Fatalf("send terminal: %v", err)
	}
	terminal.Close()
	peer.Close()
	return connection, bufio.NewReader(connection)
}

func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ufc-")
	if err != nil {
		t.Fatalf("temporary socket directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "console.sock")
}
