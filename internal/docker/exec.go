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

package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

var shellCommand = []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash; exec sh"}

type Shell struct {
	ID     string
	conn   net.Conn
	reader *bufio.Reader
}

func (c *Client) OpenShell(ctx context.Context, id string, columns, rows int) (*Shell, error) {
	body := map[string]any{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          true,
		"Env":          []string{"TERM=xterm-256color"},
		"Cmd":          shellCommand,
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := c.post(ctx, "/containers/"+id+"/exec", body, &created); err != nil {
		return nil, err
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	if err != nil {
		return nil, err
	}
	shell, err := c.attachShell(ctx, conn, created.ID, columns, rows)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return shell, nil
}

func (c *Client) attachShell(ctx context.Context, conn net.Conn, id string, columns, rows int) (*Shell, error) {
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, err
		}
	}

	payload, err := json.Marshal(map[string]any{"Detach": false, "Tty": true, "ConsoleSize": []int{rows, columns}})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiHost+"/exec/"+id+"/start", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "tcp")
	if err := request.Write(conn); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		defer response.Body.Close()
		var failure apiError
		json.NewDecoder(response.Body).Decode(&failure)
		if failure.Message == "" {
			failure.Message = response.Status
		}
		return nil, fmt.Errorf("docker: %s", failure.Message)
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return &Shell{ID: id, conn: conn, reader: reader}, nil
}

func (c *Client) ResizeShell(ctx context.Context, shell *Shell, columns, rows int) error {
	return c.post(ctx, fmt.Sprintf("/exec/%s/resize?h=%d&w=%d", shell.ID, rows, columns), nil, nil)
}

func (s *Shell) Read(buffer []byte) (int, error) {
	return s.reader.Read(buffer)
}

func (s *Shell) Write(data []byte, timeout time.Duration) error {
	if err := s.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	_, err := s.conn.Write(data)
	return err
}

func (s *Shell) Close() error {
	return s.conn.Close()
}
