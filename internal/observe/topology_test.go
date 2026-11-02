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

package observe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestTopologyJoinsDesiredAndObservedWithoutInventingReadiness(t *testing.T) {
	now := time.Now()
	p := models.Project{Name: "api-prod", Builder: "b", Runners: []string{"r", "offline"}, Services: []models.Service{{Name: "web", Image: "nginx:alpine", DependsOn: []models.Dependency{{Service: "db", Condition: "healthy"}}}, {Name: "db", Image: "postgres:17"}, {Name: "migrate", Image: "app:1", Mode: models.ServiceModeJob}}}
	s := Snapshot{At: now, Registry: "healthy", Agents: []models.Agent{{ID: "r", Name: "runner", Status: models.AgentOnline, Metrics: &models.Metrics{CollectedAt: now, ContainersAvailable: true}}, {ID: "offline", Status: models.AgentOffline}}, Projects: []models.Project{p}, Releases: []models.Release{{ID: "release", Project: p.Name, Status: models.StatusSucceeded, Spec: p}}, Containers: []models.Container{{ID: "web", AgentID: "r", Project: p.Name, Service: "web", Name: "web", State: "running", Health: "none", ObservedAt: now}, {ID: "extra", AgentID: "r", Project: p.Name, Service: "removed", Name: "old-workload", State: "running", ObservedAt: now}}}
	nodes := s.Topology(p)
	ids := map[string]bool{}
	missing, unknown, completed, unexpected, dependency := false, false, false, false, false
	for _, node := range nodes {
		if ids[node.ID] {
			t.Fatalf("duplicate node identity %q", node.ID)
		}
		ids[node.ID] = true
		if node.Parent != "" && !ids[node.Parent] {
			t.Fatalf("parent %q not emitted before %q", node.Parent, node.ID)
		}
		if node.ContainerID == "web" && node.State != "running" {
			t.Fatalf("running without a check became %s", node.State)
		}
		missing = missing || node.Service == "db" && node.AgentID == "r" && node.State == "missing"
		unknown = unknown || node.Service == "db" && node.AgentID == "offline" && node.State == "unknown"
		completed = completed || node.Service == "migrate" && node.State == "completed"
		unexpected = unexpected || node.ContainerID == "extra" && node.Relation == "unexpected observation"
		dependency = dependency || node.Kind == "depends" && node.Name == "db" && strings.Contains(node.Summary, "requires healthy")
	}
	if !missing || !unknown || !completed || !unexpected || !dependency {
		t.Fatalf("missing=%t unknown=%t completed=%t extra=%t dependency=%t", missing, unknown, completed, unexpected, dependency)
	}

	s.Agents[0].Status = models.AgentOffline
	if got := s.ContainerState(s.Containers[0], false); got != "unknown" {
		t.Fatalf("offline workload state=%s", got)
	}
}
func TestFreshnessUsesAgentCadenceAndCollectionAvailability(t *testing.T) {
	now := time.Now()
	p := models.Project{Name: "p", Runners: []string{"r"}, Services: []models.Service{{Name: "web", Image: "image"}}}
	s := Snapshot{At: now, Agents: []models.Agent{{ID: "r", Status: models.AgentOnline, Metrics: &models.Metrics{CollectedAt: now.Add(-time.Minute), MetricsSec: 60, ContainersAvailable: true}}}, Releases: []models.Release{{Project: "p", Status: models.StatusSucceeded, Spec: p}}}
	if !s.Fresh("r", now.Add(-time.Minute)) {
		t.Fatal("valid slow collector marked stale")
	}
	s.Agents[0].Metrics.ContainersAvailable = false
	if got := s.ExpectedState(p, p.Services[0], "r"); got != "unknown" {
		t.Fatalf("failed inventory treated as %s", got)
	}
	if s.Fresh("r", time.Time{}) || s.Fresh("r", now.Add(time.Hour)) {
		t.Fatal("missing or future sample accepted")
	}
	c := models.Container{AgentID: "r", State: "exited", ObservedAt: now, Details: &ufp.ContainerDetails{ExitCode: 0}}
	if got := s.ContainerState(c, true); got != "completed" {
		t.Fatalf("completed job=%s", got)
	}
	if got := s.ContainerState(c, false); got != "exited" {
		t.Fatalf("long-running service=%s", got)
	}
}
func TestSnapshotRedactionDoesNotMutateAuthoritativeConfiguration(t *testing.T) {
	p := models.Project{GitURL: "https://user:token@example.test/repo?access_token=hidden", BuildArgs: map[string]string{"TOKEN": "build-secret"}, Runtime: models.Runtime{Env: map[string]string{"PASSWORD": "plain-secret", "DB": "${secret:prod/db}"}}, Services: []models.Service{{Env: map[string]string{"KEY": "service-secret"}}}}
	safe := RedactProject(p)
	data, err := json.Marshal(safe)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"token@", "access_token", "build-secret", "plain-secret", "service-secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("snapshot exposed %q", secret)
		}
	}
	if p.Runtime.Env["PASSWORD"] != "plain-secret" || p.BuildArgs["TOKEN"] != "build-secret" || p.Services[0].Env["KEY"] != "service-secret" {
		t.Fatal("redaction mutated original maps")
	}
	if safe.Runtime.Env["DB"] != "${secret:prod/db}" {
		t.Fatal("secret reference lost")
	}
}

func TestRedactionRetainsSharedReferenceMetadataWithoutItsSurroundingValue(t *testing.T) {
	p := models.Project{Runtime: models.Runtime{Env: map[string]string{"CONNECTION": "private-prefix-${secret:shared.TOKEN}-private-suffix"}}}
	redacted := RedactProject(p).Runtime.Env["CONNECTION"]
	if !strings.Contains(redacted, "${secret:shared.TOKEN}") || strings.Contains(redacted, "private-prefix") || strings.Contains(redacted, "private-suffix") {
		t.Fatalf("unsafe or missing reference metadata: %s", redacted)
	}
}

func TestFleetGraphPreservesHostsAndAddsObservedOwnership(t *testing.T) {
	now := time.Now()
	project := models.Project{Name: "p", Builder: "builder", Runners: []string{"runner", "offline"}, Services: []models.Service{{Name: "web", Image: "image"}}}
	snapshot := Snapshot{At: now, ServerName: "control", Projects: []models.Project{project}, Agents: []models.Agent{
		{ID: "builder", Status: models.AgentOnline}, {ID: "runner", Status: models.AgentOnline, Metrics: &models.Metrics{CollectedAt: now, CPUPercent: 5}}, {ID: "offline", Status: models.AgentOffline}, {ID: "extra", Status: models.AgentOnline},
	}, Containers: []models.Container{{ID: "c", AgentID: "runner", Project: "p", Service: "web", State: "running", ObservedAt: now}, {ID: "stale", AgentID: "offline", Project: "p", Service: "web", State: "running", ObservedAt: now}, {ID: "unexpected", AgentID: "extra", Project: "p", Service: "web", State: "running", ObservedAt: now}}}
	graph := snapshot.FleetGraph()
	nodes := map[string]Node{}
	for _, node := range graph.Nodes {
		if _, exists := nodes[node.ID]; exists {
			t.Fatal("duplicate fleet node", node.ID)
		}
		if node.Parent != "" {
			if _, exists := nodes[node.Parent]; !exists {
				t.Fatal("missing parent", node.ID, node.Parent)
			}
		}
		nodes[node.ID] = node
	}
	for _, id := range []string{"server/control", "server/control/registry", "agent/builder", "agent/runner", "project/p"} {
		if _, exists := nodes[id]; !exists {
			t.Fatal("existing fleet identity changed", id)
		}
	}
	if nodes["container/runner/c"].Parent != "agent/runner" || nodes["container/offline/stale"].State != "unknown" {
		t.Fatal("observed ownership or offline state lost", nodes)
	}
	if nodes["container/extra/unexpected"].Relation != "unexpected observation" {
		t.Fatal("unexpected placement presented as configured")
	}
	if nodes["project/p"].Parent != "" {
		t.Fatal("project presented as a direct server child")
	}
	kinds := map[string]string{}
	labels := map[string]string{}
	for _, edge := range graph.Edges {
		if _, ok := nodes[edge.Source]; !ok {
			t.Fatal("missing edge source")
		}
		if _, ok := nodes[edge.Target]; !ok {
			t.Fatal("missing edge target")
		}
		if edge.Target == "project/p" {
			if !strings.HasPrefix(edge.Source, "agent/") {
				t.Fatal("project connected directly to control plane", edge)
			}
			if _, exists := kinds[edge.Source]; exists {
				t.Fatal("duplicate project placement", edge)
			}
			kinds[edge.Source] = edge.Kind
			labels[edge.Source] = edge.Label
		}
	}
	if kinds["agent/builder"] != "assignment" || kinds["agent/runner"] != "placement" || kinds["agent/extra"] != "placement" {
		t.Fatal("assigned and observed placement were mixed", kinds)
	}
	if labels["agent/builder"] != "builds" || labels["agent/runner"] != "runs" || labels["agent/offline"] != "observed" {
		t.Fatal("build, runtime and stale placement were mixed", labels)
	}
	before, _ := json.Marshal(graph)
	snapshot.Agents[1].Metrics.CPUPercent = 93
	snapshot.Containers[0].CPUPercent = 71
	snapshot.Containers[0].MemoryUsage = 1024
	after, _ := json.Marshal(snapshot.FleetGraph())
	if string(before) != string(after) {
		t.Fatal("metric updates changed graph topology data")
	}
}

func TestFleetGraphKeepsUnassignedProjectsWithoutInventingConnections(t *testing.T) {
	snapshot := Snapshot{ServerName: "control", Projects: []models.Project{{Name: "unassigned"}, {Name: "missing-runner", Runners: []string{"absent"}}}}
	graph := snapshot.FleetGraph()
	projects := 0
	for _, node := range graph.Nodes {
		if node.Kind == "project" {
			projects++
			if node.Parent != "" {
				t.Fatal("unassigned project has an invented host", node)
			}
		}
	}
	if projects != 2 {
		t.Fatal("unassigned project disappeared", graph.Nodes)
	}
	for _, edge := range graph.Edges {
		if strings.HasPrefix(edge.Target, "project/") {
			t.Fatal("unassigned project has an invented connection", edge)
		}
	}
}
