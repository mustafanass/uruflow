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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

var ErrNotFound = errors.New("docker object not found")

const (
	DefaultSocket = "/var/run/docker.sock"
	apiHost       = "http://docker"
	requestLimit  = 30 * time.Second
)

type Client struct {
	http   *http.Client
	socket string
}

type apiError struct {
	Message string `json:"message"`
}

func New(socket string) (*Client, error) {
	if socket == "" {
		socket = DefaultSocket
	}

	client := &Client{
		socket: socket,
		http: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socket)
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.get(ctx, "/version", nil); err != nil {
		return nil, fmt.Errorf("docker unavailable on %s: %w", socket, err)
	}
	return client, nil
}

func (c *Client) Socket() string { return c.socket }

func (c *Client) get(ctx context.Context, path string, out any) error {
	response, err := c.do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if out == nil {
		io.Copy(io.Discard, response.Body)
		return nil
	}
	return json.NewDecoder(response.Body).Decode(out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	response, err := c.do(ctx, http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if out == nil {
		io.Copy(io.Discard, response.Body)
		return nil
	}
	return json.NewDecoder(response.Body).Decode(out)
}

func (c *Client) delete(ctx context.Context, path string) error {
	response, err := c.do(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, headers map[string]string) (*http.Response, error) {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, apiHost+path, payload)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode >= http.StatusBadRequest {
		defer response.Body.Close()
		var failure apiError
		json.NewDecoder(response.Body).Decode(&failure)
		if failure.Message == "" {
			failure.Message = response.Status
		}
		if response.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: docker: %s", ErrNotFound, failure.Message)
		}
		return nil, fmt.Errorf("docker: %s", failure.Message)
	}

	return response, nil
}
