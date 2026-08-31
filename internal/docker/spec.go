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
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func hostConfig(spec Spec) map[string]any {
	config := map[string]any{
		"RestartPolicy": restartPolicy(spec.Restart),
	}

	if bindings := portBindings(spec.Ports); len(bindings) > 0 {
		config["PortBindings"] = bindings
	}
	if binds := mountBinds(spec.Mounts); len(binds) > 0 {
		config["Binds"] = binds
	}
	if mounts := mountSpecs(spec.Mounts); len(mounts) > 0 {
		config["Mounts"] = mounts
	}
	if spec.Network != "" {
		config["NetworkMode"] = spec.Network
	} else if len(spec.Networks) > 0 {
		config["NetworkMode"] = spec.Networks[0].Name
	}
	if spec.Resources.MemoryBytes > 0 {
		config["Memory"] = spec.Resources.MemoryBytes
	}
	if spec.Resources.CPUs > 0 {
		config["NanoCpus"] = int64(spec.Resources.CPUs * 1_000_000_000)
	}
	if spec.Resources.PIDs > 0 {
		config["PidsLimit"] = spec.Resources.PIDs
	}
	if spec.Security.NoNewPrivileges {
		config["SecurityOpt"] = []string{"no-new-privileges"}
	}
	if spec.Security.ReadOnlyRootFS {
		config["ReadonlyRootfs"] = true
	}
	if len(spec.Security.CapAdd) > 0 {
		config["CapAdd"] = spec.Security.CapAdd
	}
	if len(spec.Security.CapDrop) > 0 {
		config["CapDrop"] = spec.Security.CapDrop
	}
	if spec.Logging.Driver != "" {
		config["LogConfig"] = map[string]any{"Type": spec.Logging.Driver, "Config": spec.Logging.Options}
	}

	return config
}

func restartPolicy(policy string) map[string]any {
	if policy == "" {
		policy = "unless-stopped"
	}
	name, maximum := policy, 0
	if before, after, found := strings.Cut(policy, ":"); found {
		name = before
		maximum, _ = strconv.Atoi(after)
	}
	return map[string]any{"Name": name, "MaximumRetryCount": maximum}
}

func environment(env map[string]string) []string {
	values := make([]string, 0, len(env))
	for key, value := range env {
		values = append(values, key+"="+value)
	}
	sort.Strings(values)
	return values
}

func portKey(port PortBinding) string {
	protocol := port.Protocol
	if protocol == "" {
		protocol = "tcp"
	}
	return fmt.Sprintf("%d/%s", port.Container, protocol)
}

func exposedPorts(ports []PortBinding) map[string]any {
	exposed := make(map[string]any, len(ports))
	for _, port := range ports {
		exposed[portKey(port)] = struct{}{}
	}
	return exposed
}

func portBindings(ports []PortBinding) map[string]any {
	bindings := make(map[string]any, len(ports))
	for _, port := range ports {
		bindings[portKey(port)] = []map[string]string{
			{"HostIp": port.HostIP, "HostPort": fmt.Sprint(port.Host)},
		}
	}
	return bindings
}

func networkEndpoints(spec Spec) map[string]any {
	attachments := spec.Networks
	if len(attachments) == 0 && spec.Network != "" {
		attachments = []NetworkAttachment{{Name: spec.Network}}
	}
	endpoints := make(map[string]any, len(attachments))
	for _, network := range attachments {
		if network.Name == "" {
			continue
		}
		endpoints[network.Name] = map[string]any{"Aliases": network.Aliases}
	}
	return endpoints
}

func mountBinds(mounts []Mount) []string {
	binds := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		if mount.Type != "" {
			continue
		}
		bind := mount.Source + ":" + mount.Target
		if mount.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}
	return binds
}

func mountSpecs(mounts []Mount) []map[string]any {
	result := make([]map[string]any, 0, len(mounts))
	for _, mount := range mounts {
		if mount.Type == "" {
			continue
		}
		entry := map[string]any{"Type": mount.Type, "Target": mount.Target, "ReadOnly": mount.ReadOnly}
		if mount.Source != "" {
			entry["Source"] = mount.Source
		}
		if mount.Type == "bind" {
			entry["BindOptions"] = map[string]any{"CreateMountpoint": mount.CreateHostPath}
		}
		result = append(result, entry)
	}
	return result
}

func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}
