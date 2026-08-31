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
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

const pollInterval = 500 * time.Millisecond

func (c *Client) Create(ctx context.Context, spec Spec) (string, error) {
	body := map[string]any{
		"Image":      spec.Image,
		"Labels":     spec.Labels,
		"HostConfig": hostConfig(spec),
	}
	if len(spec.Entrypoint) > 0 {
		body["Entrypoint"] = spec.Entrypoint
	}

	if len(spec.Command) > 0 {
		body["Cmd"] = spec.Command
	}
	if len(spec.Env) > 0 {
		body["Env"] = environment(spec.Env)
	}
	if spec.Security.User != "" {
		body["User"] = spec.Security.User
	}
	if spec.Healthcheck != nil {
		body["Healthcheck"] = map[string]any{
			"Test": spec.Healthcheck.Test, "Interval": spec.Healthcheck.Interval.Nanoseconds(),
			"Timeout": spec.Healthcheck.Timeout.Nanoseconds(), "Retries": spec.Healthcheck.Retries,
			"StartPeriod": spec.Healthcheck.StartPeriod.Nanoseconds(),
		}
	}
	if endpoints := networkEndpoints(spec); len(endpoints) > 0 {
		body["NetworkingConfig"] = map[string]any{"EndpointsConfig": endpoints}
	}
	if exposed := exposedPorts(spec.Ports); len(exposed) > 0 {
		body["ExposedPorts"] = exposed
	}

	var created struct {
		ID string `json:"Id"`
	}
	path := "/containers/create?name=" + url.QueryEscape(spec.Name)
	if err := c.post(ctx, path, body, &created); err != nil {
		return "", err
	}
	return created.ID, nil
}

func (c *Client) Start(ctx context.Context, id string) error {
	return c.post(ctx, "/containers/"+id+"/start", nil, nil)
}

func (c *Client) Stop(ctx context.Context, id string, timeout time.Duration) error {
	return c.post(ctx, fmt.Sprintf("/containers/%s/stop?t=%d", id, int(timeout.Seconds())), nil, nil)
}

func (c *Client) Remove(ctx context.Context, id string, force bool) error {
	return c.delete(ctx, fmt.Sprintf("/containers/%s?force=%t&v=true", id, force))
}

func (c *Client) Rename(ctx context.Context, id, name string) error {
	return c.post(ctx, "/containers/"+id+"/rename?name="+url.QueryEscape(name), nil, nil)
}

func (c *Client) State(ctx context.Context, id string) (*State, error) {
	details, err := c.Inspect(ctx, id)
	if err != nil {
		return nil, err
	}

	state := &State{
		Status:   details.State.Status,
		Health:   HealthNone,
		ExitCode: details.State.ExitCode,
		Restarts: details.RestartCount,
	}
	if details.State.Health != nil && details.State.Health.Status != "" {
		state.Health = details.State.Health.Status
	}
	return state, nil
}

func (s *State) Failed() error {
	switch {
	case s.Status == StateExited || s.Status == StateDead:
		return fmt.Errorf("container exited with code %d", s.ExitCode)
	case s.Health == HealthUnhealthy:
		return errors.New("container reported itself unhealthy")
	default:
		return nil
	}
}

func (s *State) Ready() bool {
	if s.Status != StateRunning {
		return false
	}
	return s.Health == HealthHealthy || s.Health == HealthNone
}

func (c *Client) WaitReady(ctx context.Context, id string, settle, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var readySince time.Time
	restarts := -1

	for time.Now().Before(deadline) {
		state, err := c.State(ctx, id)
		if err != nil {
			return err
		}
		if err := state.Failed(); err != nil {
			return err
		}

		if restarts < 0 {
			restarts = state.Restarts
		}
		if state.Restarts > restarts {
			return fmt.Errorf("container restarted %d time(s) while coming up", state.Restarts-restarts)
		}

		switch {
		case !state.Ready():
			readySince = time.Time{}
		case state.Health == HealthHealthy:
			return nil
		case readySince.IsZero():
			readySince = time.Now()
		case time.Since(readySince) >= settle:
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return fmt.Errorf("container did not become ready within %s", timeout)
}

func (c *Client) Run(ctx context.Context, spec Spec) (string, error) {
	id, err := c.Create(ctx, spec)
	if err != nil {
		return "", err
	}
	if err := c.Start(ctx, id); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), requestLimit)
		cleanupErr := c.Remove(cleanupCtx, id, true)
		cancel()
		if cleanupErr != nil {
			return "", errors.Join(err, fmt.Errorf("remove failed container: %w", cleanupErr))
		}
		return "", err
	}
	return id, nil
}
