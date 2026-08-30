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
	"net"
	"os"
	"syscall"

	"github.com/mustafanass/uruflow/internal/ops"
)

type Client struct {
	path string
}

func NewClient(path string) *Client { return &Client{path: path} }

func (c *Client) Execute(ctx context.Context, args []string, input string, emit ops.Emit) error {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", c.path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) {
			return errors.New("connect to uruflow: permission denied (run with sudo)")
		}
		return fmt.Errorf("uruflow server is not running (control socket %s): %w", c.path, err)
	}
	defer connection.Close()
	if err := json.NewEncoder(connection).Encode(Request{Args: args, Input: input}); err != nil {
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			connection.Close()
		case <-done:
		}
	}()
	decoder := json.NewDecoder(bufio.NewReader(connection))
	for {
		var response Response
		if err := decoder.Decode(&response); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("control stream: %w", err)
		}
		if response.Error != "" {
			return errors.New(response.Error)
		}
		if response.Event != nil && emit != nil {
			if err := emit(*response.Event); err != nil {
				return err
			}
		}
		if response.Done {
			return nil
		}
	}
}
