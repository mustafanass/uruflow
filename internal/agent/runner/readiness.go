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

package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	inspectTimeout = 15 * time.Second
	settleWindow   = 5 * time.Second
	readyTimeout   = 2 * time.Minute
	readinessPoll  = 100 * time.Millisecond
	shortIDSize    = 12
)

func (r *Runner) waitReady(ctx context.Context, id string, healthcheck *ufp.HealthcheckSpec) error {
	if healthcheck == nil {
		return r.docker.WaitReady(ctx, id, settleWindow, readyTimeout)
	}
	if healthcheck.Type == "running" {
		return r.waitRunning(ctx, id, healthcheck.StableFor)
	}
	if healthcheck.Type == "command" {
		return r.docker.WaitReady(ctx, id, settleWindow, readyTimeout)
	}
	return r.waitProbe(ctx, id, healthcheck)
}

func (r *Runner) waitRunning(ctx context.Context, id string, stableFor time.Duration) error {
	var runningSince time.Time
	restarts := -1
	for {
		inspectCtx, cancel := context.WithTimeout(ctx, inspectTimeout)
		state, err := r.docker.State(inspectCtx, id)
		cancel()
		if err != nil {
			return err
		}
		if err := runtimeFailure(state); err != nil {
			return err
		}
		if restarts < 0 {
			restarts = state.Restarts
		}
		if state.Restarts > restarts {
			return fmt.Errorf("container restarted %d time(s) while coming up", state.Restarts-restarts)
		}
		if state.Status != docker.StateRunning {
			runningSince = time.Time{}
		} else if runningSince.IsZero() {
			runningSince = time.Now()
		} else if time.Since(runningSince) >= stableFor {
			return nil
		}
		if err := wait(ctx, readinessPoll); err != nil {
			return err
		}
	}
}

func (r *Runner) waitProbe(ctx context.Context, id string, healthcheck *ufp.HealthcheckSpec) error {
	if healthcheck.StartPeriod > 0 {
		if err := wait(ctx, healthcheck.StartPeriod); err != nil {
			return err
		}
	}
	restarts := -1
	var lastErr error
	for attempt := 1; attempt <= healthcheck.Retries; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, healthcheck.Timeout)
		state, err := r.docker.State(attemptCtx, id)
		if err != nil {
			cancel()
			return err
		}
		if err := runtimeFailure(state); err != nil {
			cancel()
			return err
		}
		if restarts < 0 {
			restarts = state.Restarts
		}
		if state.Restarts > restarts {
			cancel()
			return fmt.Errorf("container restarted %d time(s) while coming up", state.Restarts-restarts)
		}

		endpoint, err := r.docker.Endpoint(attemptCtx, id, healthcheck.Port)
		if err == nil {
			switch healthcheck.Type {
			case "http":
				err = probeHTTP(attemptCtx, healthcheck.Scheme, endpoint, healthcheck.Path)
			case "tcp":
				err = probeTCP(attemptCtx, endpoint)
			default:
				err = fmt.Errorf("unsupported healthcheck type %q", healthcheck.Type)
			}
		}
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < healthcheck.Retries {
			if err := wait(ctx, healthcheck.Interval); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("%s healthcheck failed after %d attempt(s): %w", healthcheck.Type, healthcheck.Retries, lastErr)
}

func runtimeFailure(state *docker.State) error {
	if state.Status == docker.StateExited || state.Status == docker.StateDead {
		return fmt.Errorf("container exited with code %d", state.ExitCode)
	}
	return nil
}

func probeTCP(ctx context.Context, endpoint string) error {
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return err
	}
	return connection.Close()
}

func probeHTTP(ctx context.Context, scheme, endpoint, path string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+endpoint+path, nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("http status %d", response.StatusCode)
	}
	return nil
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func shortID(id string) string {
	if len(id) <= shortIDSize {
		return id
	}
	return id[:shortIDSize]
}
