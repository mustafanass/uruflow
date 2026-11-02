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

import "github.com/mustafanass/uruflow/internal/models"

type Edge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
}
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}
type ProjectGraphs struct {
	Runtime  Graph `json:"runtime"`
	Delivery Graph `json:"delivery"`
}

func (s Snapshot) ProjectGraphs(p models.Project) ProjectGraphs {
	nodes := s.Topology(p)
	runtime, delivery := Graph{Nodes: []Node{}, Edges: []Edge{}}, Graph{Nodes: []Node{}, Edges: []Edge{}}
	runtimeIDs, deliveryIDs := map[string]bool{}, map[string]bool{}
	root := "project/" + p.Name
	for _, n := range nodes {
		switch n.Kind {
		case "depends":
			continue
		case "source", "builder", "registry", "image":
			delivery.Nodes = append(delivery.Nodes, n)
			deliveryIDs[n.ID] = true
		case "network", "mount", "volume":
			runtime.Nodes = append(runtime.Nodes, n)
			runtimeIDs[n.ID] = true
		default:
			runtime.Nodes = append(runtime.Nodes, n)
			runtimeIDs[n.ID] = true
			if n.Kind != "container" {
				delivery.Nodes = append(delivery.Nodes, n)
				deliveryIDs[n.ID] = true
			}
		}
	}
	appendEdge := func(g *Graph, ids map[string]bool, source, target, kind, label string) {
		if ids[source] && ids[target] {
			g.Edges = append(g.Edges, Edge{ID: source + "→" + target + "/" + kind, Source: source, Target: target, Kind: kind, Label: label})
		}
	}
	for _, n := range runtime.Nodes {
		appendEdge(&runtime, runtimeIDs, n.Parent, n.ID, n.Relation, "")
	}
	for _, service := range p.ServiceList() {
		id := root + "/service/" + service.Name
		for _, dep := range service.DependsOn {
			appendEdge(&runtime, runtimeIDs, id, root+"/service/"+dep.Service, "dependency", "requires "+dep.Condition)
		}
		appendEdge(&delivery, deliveryIDs, root, id, "owns", "")
		if service.Built() {
			appendEdge(&delivery, deliveryIDs, id+"/source", root+"/builder", "build", "build input")
			appendEdge(&delivery, deliveryIDs, root+"/builder", id+"/image", "build", "produces")
		}
		appendEdge(&delivery, deliveryIDs, id, id+"/image", "artifact", "")
		appendEdge(&delivery, deliveryIDs, id+"/image", root+"/registry", "registry", "stored artifact")
		for _, runner := range p.Runners {
			placement := id + "/runner/" + runner
			appendEdge(&delivery, deliveryIDs, root+"/registry", placement, "release", "pull / release")
			appendEdge(&delivery, deliveryIDs, id, placement, "placement", "")
			appendEdge(&delivery, deliveryIDs, placement, placement+"/expected", "result", "")
		}
	}
	return ProjectGraphs{Runtime: runtime, Delivery: delivery}
}
func (s Snapshot) FleetGraph() Graph {
	nodes := s.FleetTopology()
	g := Graph{Nodes: nodes, Edges: []Edge{}}
	for _, n := range nodes {
		if n.Parent != "" {
			g.Edges = append(g.Edges, Edge{ID: n.Parent + "→" + n.ID, Source: n.Parent, Target: n.ID, Kind: n.Relation})
		}
	}
	ids := map[string]bool{}
	for _, node := range nodes {
		ids[node.ID] = true
	}
	type connection struct{ source, target string }
	observed := map[connection]bool{}
	running := map[connection]bool{}
	hosts := map[string][]string{}
	for _, container := range s.Containers {
		link := connection{"agent/" + container.AgentID, "project/" + container.Project}
		observed[link] = true
		if container.State == "running" && s.Fresh(container.AgentID, container.ObservedAt) {
			running[link] = true
		}
		hosts[container.Project] = append(hosts[container.Project], container.AgentID)
	}
	for _, project := range s.Projects {
		machines := append([]string{}, project.Runners...)
		runners := map[string]bool{}
		for _, runner := range project.Runners {
			runners[runner] = true
		}
		if project.Builder != "" {
			machines = append(machines, project.Builder)
		}
		machines = append(machines, hosts[project.Name]...)
		seen := map[string]bool{}
		for _, agent := range machines {
			source, target := "agent/"+agent, "project/"+project.Name
			if seen[agent] || !ids[source] || !ids[target] {
				continue
			}
			seen[agent] = true
			kind, label := "assignment", "assigned"
			if observed[connection{source, target}] {
				kind, label = "placement", "observed"
				if running[connection{source, target}] {
					label = "runs"
				}
			}
			if agent == project.Builder {
				if runners[agent] || observed[connection{source, target}] {
					label = "builds / " + label
				} else {
					label = "builds"
				}
			}
			g.Edges = append(g.Edges, Edge{ID: source + "→" + target + "/placement", Source: source, Target: target, Kind: kind, Label: label})
		}
	}
	return g
}
