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
	"fmt"
	"strconv"
	"strings"
)

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
