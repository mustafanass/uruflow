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

package models

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var resourceNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

func ValidResourceName(value string) bool {
	return resourceNamePattern.MatchString(value)
}

func ValidSourcePath(value string) bool {
	if value == "" {
		return true
	}
	cleaned := filepath.Clean(value)
	return !filepath.IsAbs(value) && cleaned != ".." &&
		!strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

func ValidDigestReference(value string) bool {
	repository, digest, found := strings.Cut(value, "@sha256:")
	if !found || repository == "" || repository != strings.ToLower(repository) || strings.ContainsAny(repository, "@ \t\r\n") ||
		strings.HasSuffix(repository, "/") || len(digest) != 64 || digest != strings.ToLower(digest) {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

func ValidGitCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 {
		return false
	}
	_, err := hex.DecodeString(commit)
	return err == nil
}

func ValidateHealthcheck(healthcheck *Healthcheck) error {
	if healthcheck == nil {
		return nil
	}
	switch healthcheck.Type {
	case "http":
		parsed, err := url.ParseRequestURI(healthcheck.Path)
		if err != nil || !strings.HasPrefix(healthcheck.Path, "/") || parsed.IsAbs() || parsed.Host != "" || parsed.Fragment != "" {
			return fmt.Errorf("healthcheck.path must be a valid absolute request path")
		}
		if healthcheck.Scheme != "http" && healthcheck.Scheme != "https" {
			return fmt.Errorf("healthcheck.scheme must be http or https")
		}
		if healthcheck.StableFor != 0 {
			return fmt.Errorf("healthcheck.stable_for is only valid for running")
		}
		return validateProbeHealthcheck(healthcheck)
	case "tcp":
		if healthcheck.Scheme != "" || healthcheck.Path != "" || healthcheck.StableFor != 0 {
			return fmt.Errorf("healthcheck contains fields that are not valid for tcp")
		}
		return validateProbeHealthcheck(healthcheck)
	case "running":
		if healthcheck.StableFor <= 0 {
			return fmt.Errorf("healthcheck.stable_for must be positive")
		}
		if healthcheck.Scheme != "" || healthcheck.Path != "" || healthcheck.Port != 0 || healthcheck.Interval != 0 || healthcheck.Timeout != 0 || healthcheck.Retries != 0 {
			return fmt.Errorf("healthcheck contains fields that are not valid for running")
		}
	default:
		return fmt.Errorf("healthcheck.type %q is not supported", healthcheck.Type)
	}
	return nil
}

func validateProbeHealthcheck(healthcheck *Healthcheck) error {
	if healthcheck.Port < 1 || healthcheck.Port > 65535 {
		return fmt.Errorf("healthcheck.port must be between 1 and 65535")
	}
	if healthcheck.Interval <= 0 {
		return fmt.Errorf("healthcheck.interval must be positive")
	}
	if healthcheck.Timeout <= 0 {
		return fmt.Errorf("healthcheck.timeout must be positive")
	}
	if healthcheck.Retries <= 0 {
		return fmt.Errorf("healthcheck.retries must be positive")
	}
	return nil
}

func ValidateLabels(labels map[string]string) error {
	for key := range labels {
		if key == "" || strings.TrimSpace(key) != key || strings.ContainsAny(key, " \t\r\n") {
			return fmt.Errorf("label %q has an invalid key", key)
		}
		if strings.HasPrefix(key, "uruflow.") {
			return fmt.Errorf("label %q is reserved for uruflow", key)
		}
	}
	return nil
}

const (
	protocolTCP  = "tcp"
	protocolUDP  = "udp"
	readOnlyFlag = "ro"
)

func parsePort(entry string) (Port, error) {
	spec, protocol := entry, ""
	if base, suffix, found := strings.Cut(entry, "/"); found {
		spec, protocol = base, strings.ToLower(strings.TrimSpace(suffix))
		if protocol != protocolUDP && protocol != protocolTCP {
			return Port{}, fmt.Errorf("port %q has an unknown protocol %q", entry, suffix)
		}
	}

	host, container, found := strings.Cut(spec, ":")
	if !found {
		return Port{}, fmt.Errorf("port %q must be host:container", entry)
	}

	hostPort, err := strconv.Atoi(strings.TrimSpace(host))
	if err != nil {
		return Port{}, fmt.Errorf("port %q has an invalid host port", entry)
	}
	containerPort, err := strconv.Atoi(strings.TrimSpace(container))
	if err != nil {
		return Port{}, fmt.Errorf("port %q has an invalid container port", entry)
	}
	if hostPort < 0 || hostPort > 65535 || containerPort < 1 || containerPort > 65535 {
		return Port{}, fmt.Errorf("port %q is outside the valid range", entry)
	}

	return Port{Host: hostPort, Container: containerPort, Protocol: protocol}, nil
}

func ParsePorts(entries []string) ([]Port, error) {
	ports := make([]Port, 0, len(entries))
	for _, entry := range entries {
		port, err := parsePort(entry)
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func formatPort(port Port) string {
	entry := fmt.Sprintf("%d:%d", port.Host, port.Container)
	if port.Protocol != "" && port.Protocol != protocolTCP {
		entry += "/" + port.Protocol
	}
	return entry
}

func FormatPorts(ports []Port) []string {
	entries := make([]string, 0, len(ports))
	for _, port := range ports {
		entries = append(entries, formatPort(port))
	}
	return entries
}

func parseVolume(entry string) (Volume, error) {
	parts := strings.Split(entry, ":")
	if len(parts) < 2 {
		return Volume{}, fmt.Errorf("volume %q must be source:target", entry)
	}

	volume := Volume{Source: strings.TrimSpace(parts[0]), Target: strings.TrimSpace(parts[1])}
	if len(parts) > 2 {
		if strings.TrimSpace(parts[2]) != readOnlyFlag {
			return Volume{}, fmt.Errorf("volume %q accepts only %q as a third field", entry, readOnlyFlag)
		}
		volume.ReadOnly = true
	}
	return volume, nil
}

func ParseVolumes(entries []string) ([]Volume, error) {
	volumes := make([]Volume, 0, len(entries))
	for _, entry := range entries {
		volume, err := parseVolume(entry)
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, volume)
	}
	return volumes, nil
}

func formatVolume(volume Volume) string {
	entry := volume.Source + ":" + volume.Target
	if volume.ReadOnly {
		entry += ":" + readOnlyFlag
	}
	return entry
}

func FormatVolumes(volumes []Volume) []string {
	entries := make([]string, 0, len(volumes))
	for _, volume := range volumes {
		entries = append(entries, formatVolume(volume))
	}
	return entries
}
