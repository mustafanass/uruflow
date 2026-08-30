/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 * This file is part of uruflow and is licensed under the MIT License.
 */

package docker

import "testing"

func TestNativeHostConfig(t *testing.T) {
	spec := Spec{
		Networks:  []NetworkAttachment{{Name: "edge", Aliases: []string{"api"}}, {Name: "data"}},
		Ports:     []PortBinding{{HostIP: "127.0.0.1", Host: 8082, Container: 8082}},
		Resources: ResourceLimits{MemoryBytes: 256 << 20, CPUs: 1.5, PIDs: 128},
		Security:  Security{NoNewPrivileges: true, ReadOnlyRootFS: true, CapDrop: []string{"ALL"}},
		Logging:   LogConfig{Driver: "json-file", Options: map[string]string{"max-size": "10m"}},
	}
	host := hostConfig(spec)
	if host["NetworkMode"] != "edge" || host["Memory"] != int64(256<<20) || host["NanoCpus"] != int64(1_500_000_000) {
		t.Fatalf("host config = %#v", host)
	}
	bindings := host["PortBindings"].(map[string]any)["8082/tcp"].([]map[string]string)
	if bindings[0]["HostIp"] != "127.0.0.1" {
		t.Fatalf("bindings = %#v", bindings)
	}
	if networkEndpoints(spec)["edge"] == nil {
		t.Fatalf("endpoints = %#v", networkEndpoints(spec))
	}
}

func TestTypedMountsStayOutOfLegacyBinds(t *testing.T) {
	mounts := []Mount{{Source: "/legacy", Target: "/legacy"}, {Type: "bind", Source: "/config", Target: "/app/config", CreateHostPath: false}, {Type: "tmpfs", Target: "/tmp"}}
	if got := mountBinds(mounts); len(got) != 1 {
		t.Fatalf("binds = %#v", got)
	}
	if got := mountSpecs(mounts); len(got) != 2 {
		t.Fatalf("mount specs = %#v", got)
	}
}
