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

package projects

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

const (
	defaultHealthInterval = 5 * time.Second
	defaultHealthTimeout  = 3 * time.Second
	defaultHealthRetries  = 10
)

func buildHealthcheck(service string, declaration *Healthcheck) (*models.Healthcheck, error) {
	if declaration == nil {
		return nil, nil
	}

	field := func(name string) string { return fmt.Sprintf("service %q healthcheck.%s", service, name) }
	healthcheck := &models.Healthcheck{Type: declaration.Type}
	switch declaration.Type {
	case "http", "tcp", "command":
		if declaration.Type != "command" && (declaration.Port < 1 || declaration.Port > 65535) {
			return nil, fmt.Errorf("%s must be between 1 and 65535", field("port"))
		}
		healthcheck.Port = declaration.Port
		healthcheck.Interval = defaultHealthInterval
		healthcheck.Timeout = defaultHealthTimeout
		healthcheck.Retries = defaultHealthRetries
		var err error
		if declaration.StartPeriod != "" {
			healthcheck.StartPeriod, err = positiveDuration(field("start_period"), declaration.StartPeriod)
			if err != nil {
				return nil, err
			}
		}

		if declaration.Interval != "" {
			healthcheck.Interval, err = positiveDuration(field("interval"), declaration.Interval)
			if err != nil {
				return nil, err
			}
		}
		if declaration.Timeout != "" {
			healthcheck.Timeout, err = positiveDuration(field("timeout"), declaration.Timeout)
			if err != nil {
				return nil, err
			}
		}
		if declaration.Retries != nil {
			if *declaration.Retries <= 0 {
				return nil, fmt.Errorf("%s must be positive", field("retries"))
			}
			healthcheck.Retries = *declaration.Retries
		}

		if declaration.Type == "command" {
			if declaration.Port != 0 || declaration.Path != "" || declaration.Scheme != "" || declaration.StableFor != "" {
				return nil, fmt.Errorf("service %q healthcheck contains fields that are not valid for command", service)
			}
			if declaration.Command.Shell != "" {
				healthcheck.Command = []string{"CMD-SHELL", declaration.Command.Shell}
				healthcheck.Shell = true
			} else if len(declaration.Command.Exec) > 0 {
				healthcheck.Command = append([]string{"CMD"}, declaration.Command.Exec...)
			} else {
				return nil, fmt.Errorf("%s is required", field("command"))
			}
			return healthcheck, nil
		}

		if declaration.Type == "tcp" {
			if declaration.Path != "" || declaration.Scheme != "" || declaration.StableFor != "" {
				return nil, fmt.Errorf("service %q healthcheck contains fields that are not valid for tcp", service)
			}
			return healthcheck, nil
		}

		if declaration.Path == "" {
			return nil, fmt.Errorf("%s is required", field("path"))
		}
		parsed, err := url.ParseRequestURI(declaration.Path)
		if err != nil || !strings.HasPrefix(declaration.Path, "/") || parsed.IsAbs() || parsed.Host != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("%s must be a valid absolute request path", field("path"))
		}
		healthcheck.Path = declaration.Path
		healthcheck.Scheme = declaration.Scheme
		if healthcheck.Scheme == "" {
			healthcheck.Scheme = "http"
		}
		if healthcheck.Scheme != "http" && healthcheck.Scheme != "https" {
			return nil, fmt.Errorf("%s must be http or https", field("scheme"))
		}
		if declaration.StableFor != "" {
			return nil, fmt.Errorf("%s is only valid for running healthchecks", field("stable_for"))
		}
		return healthcheck, nil

	case "running":
		if declaration.StableFor == "" {
			return nil, fmt.Errorf("%s is required", field("stable_for"))
		}
		stableFor, err := positiveDuration(field("stable_for"), declaration.StableFor)
		if err != nil {
			return nil, err
		}
		if declaration.Port != 0 || declaration.Path != "" || declaration.Scheme != "" || declaration.Interval != "" || declaration.Timeout != "" || declaration.Retries != nil {
			return nil, fmt.Errorf("service %q healthcheck contains fields that are not valid for running", service)
		}
		healthcheck.StableFor = stableFor
		return healthcheck, nil

	default:
		if declaration.Type == "" {
			return nil, fmt.Errorf("%s is required", field("type"))
		}
		return nil, fmt.Errorf("%s %q is not supported", field("type"), declaration.Type)
	}
}
