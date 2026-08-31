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

import "testing"

func TestParseMemory(t *testing.T) {
	for input, want := range map[string]int64{"256M": 256 << 20, "1GiB": 1 << 30, "500MB": 500_000_000, "1024": 1024} {
		got, err := ParseMemory(input)
		if err != nil || got != want {
			t.Errorf("ParseMemory(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
	if _, err := ParseMemory("zero"); err == nil {
		t.Fatal("invalid memory was accepted")
	}
}

func TestOrderServicesRespectsDependenciesAndJobs(t *testing.T) {
	services := []Service{
		{Name: "api", DependsOn: []Dependency{{Service: "migrate", Condition: DependencyCompleted}}},
		{Name: "db"},
		{Name: "migrate", Mode: ServiceModeJob, DependsOn: []Dependency{{Service: "db", Condition: DependencyHealthy}}},
	}
	ordered, err := OrderServices(services)
	if err != nil {
		t.Fatal(err)
	}
	if ordered[0].Name != "db" || ordered[1].Name != "migrate" || ordered[2].Name != "api" {
		t.Fatalf("order = %#v", ordered)
	}

	services[1].DependsOn = []Dependency{{Service: "api", Condition: DependencyStarted}}
	if _, err := OrderServices(services); err == nil {
		t.Fatal("dependency cycle was accepted")
	}
}

func TestPortBindingAcceptsHostIP(t *testing.T) {
	ports, err := ParsePorts([]string{"127.0.0.1:8082:8082", "443:443/tcp"})
	if err != nil {
		t.Fatal(err)
	}
	if ports[0].HostIP != "127.0.0.1" || FormatPorts(ports)[0] != "127.0.0.1:8082:8082" {
		t.Fatalf("ports = %#v", ports)
	}
	if _, err := ParsePorts([]string{"not-an-ip:80:80"}); err == nil {
		t.Fatal("invalid host IP was accepted")
	}
}

func TestProjectWorkflowCapabilities(t *testing.T) {
	cases := []struct {
		name         string
		project      Project
		workflow     string
		needsBuilder bool
		needsRunners bool
	}{
		{name: "explicit build only", project: Project{Workflow: WorkflowBuildOnly}, workflow: WorkflowBuildOnly, needsBuilder: true},
		{name: "explicit release only", project: Project{Workflow: WorkflowDeployOnly}, workflow: WorkflowDeployOnly, needsRunners: true},
		{name: "legacy build and release", project: Project{Runners: []string{"runner"}}, workflow: WorkflowBuildDeploy, needsBuilder: true, needsRunners: true},
		{name: "legacy prebuilt release", project: Project{Runners: []string{"runner"}, Services: []Service{{Name: "api", Image: "api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}, workflow: WorkflowDeployOnly, needsRunners: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := test.project.EffectiveWorkflow(); got != test.workflow {
				t.Fatalf("workflow = %q, want %q", got, test.workflow)
			}
			if test.project.NeedsBuilder() != test.needsBuilder || test.project.NeedsRunners() != test.needsRunners {
				t.Fatalf("capabilities = builder:%v runners:%v", test.project.NeedsBuilder(), test.project.NeedsRunners())
			}
		})
	}
}
