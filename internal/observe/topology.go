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
	"fmt"
	"sort"
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
)

type Node struct {
	ID          string `json:"id"`
	Parent      string `json:"parent"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Summary     string `json:"summary"`
	Relation    string `json:"relation"`
	AgentID     string `json:"agent_id"`
	ContainerID string `json:"container_id"`
	Service     string `json:"service"`
}

func (s Snapshot) Topology(p models.Project) []Node {
	root := "project/" + p.Name
	nodes := []Node{{ID: root, Kind: "project", Name: p.Name, State: s.ProjectState(p), Summary: p.Env + " / " + p.EffectiveWorkflow(), Relation: "authoritative project"}}
	latest := s.LatestRelease(p.Name)
	if p.Builder != "" {
		state := "unknown"
		if a := s.Agent(p.Builder); a != nil {
			state = string(a.Status)
		}
		nodes = append(nodes, Node{ID: root + "/builder", Parent: root, Kind: "builder", Name: s.AgentName(p.Builder), State: state, Summary: "build once → registry → runners", AgentID: p.Builder, Relation: "configured builder"})
	}
	nodes = append(nodes, Node{ID: root + "/registry", Parent: root, Kind: "registry", Name: s.RegistryAddress, State: s.Registry, Summary: "immutable deployment artifacts", Relation: "artifact distribution"})
	seen := map[string]bool{}
	for _, service := range p.ServiceList() {
		name := service.Name
		if name == "" {
			name = p.Name
		}
		id := root + "/service/" + service.Name
		nodes = append(nodes, Node{ID: id, Parent: root, Kind: service.EffectiveMode(), Name: name, State: s.ServiceState(p, service), Summary: strings.Join(models.FormatPorts(service.Ports), ", "), Service: service.Name, Relation: "declared service"})
		if service.Built() {
			source := service.GitURL
			if source == "" {
				source = p.GitURL
			}
			branch := service.Branch
			if branch == "" {
				branch = p.Branch
			}
			nodes = append(nodes, Node{ID: id + "/source", Parent: id, Kind: "source", Name: SafeURL(source), State: branch, Summary: service.BuildFile() + " → " + s.AgentName(p.Builder), Service: service.Name, Relation: "build input"})
		}
		image := service.Image
		if latest != nil {
			if resolved := latest.Images[service.Name]; resolved != "" {
				image = resolved
			} else if service.Name == "" && latest.Image != "" {
				image = latest.Image
			}
		}
		if image != "" {
			nodes = append(nodes, Node{ID: id + "/image", Parent: id, Kind: "image", Name: image, Service: service.Name, Relation: "release artifact (latest attempt)"})
		}
		for _, dep := range service.DependsOn {
			nodes = append(nodes, Node{ID: id + "/depends/" + dep.Service, Parent: id, Kind: "depends", Name: dep.Service, State: "declared", Service: dep.Service, Relation: "declared dependency", Summary: "requires " + dep.Condition + " / not observed traffic"})
		}
		for _, runner := range p.Runners {
			placement := id + "/runner/" + runner
			a := s.Agent(runner)
			state := "unknown"
			if a != nil {
				state = string(a.Status)
			}
			nodes = append(nodes, Node{ID: placement, Parent: id, Kind: "runner", Name: s.AgentName(runner), State: state, AgentID: runner, Service: service.Name, Relation: "intended placement"})
			found := false
			for _, c := range s.Containers {
				if c.Project != p.Name || c.Service != service.Name || c.AgentID != runner {
					continue
				}
				found = true
				seen[c.AgentID+"/"+c.ID] = true
				nodes = append(nodes, containerNodes(s, c, placement, service.EffectiveMode() == models.ServiceModeJob)...)
			}
			if !found {
				state := s.ExpectedState(p, service, runner)
				name, summary := "No observed container", "waiting for a complete inventory from this runner"
				if state == "completed" {
					name = "Job completed"
					summary = "successful release; runner removes completed job containers"
				}
				nodes = append(nodes, Node{ID: placement + "/expected", Parent: placement, Kind: "instance", Name: name, State: state, Service: service.Name, AgentID: runner, Relation: "expected placement", Summary: summary})
			}
		}
		for _, net := range service.Networks {
			nodes = append(nodes, Node{ID: id + "/network/" + net.Name, Parent: id, Kind: "network", Name: net.Name, State: "declared", Service: service.Name, Relation: "desired attachment"})
		}
		if service.Network != "" {
			nodes = append(nodes, Node{ID: id + "/network/" + service.Network, Parent: id, Kind: "network", Name: service.Network, State: "declared", Relation: "desired attachment"})
		}
		for i, mount := range service.Volumes {
			nodes = append(nodes, Node{ID: fmt.Sprintf("%s/mount/%d", id, i), Parent: id, Kind: "mount", Name: mount.Source + " → " + mount.Target, State: "declared", Service: service.Name, Relation: "desired mount"})
		}
	}
	for _, c := range s.Containers {
		if c.Project == p.Name && !seen[c.AgentID+"/"+c.ID] {
			extra := containerNodes(s, c, root, false)
			extra[0].Relation = "unexpected observation"
			extra[0].Summary = "not in current placement / " + s.AgentName(c.AgentID)
			nodes = append(nodes, extra...)
		}
	}
	keys := make([]string, 0, len(p.Networks))
	for name := range p.Networks {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		nodes = append(nodes, Node{ID: root + "/network/" + name, Parent: root, Kind: "network", Name: name, State: "declared", Relation: "project network"})
	}
	keys = nil
	for name := range p.Volumes {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		nodes = append(nodes, Node{ID: root + "/volume/" + name, Parent: root, Kind: "volume", Name: name, State: "declared", Relation: "project volume"})
	}
	return nodes
}
func containerNodes(s Snapshot, c models.Container, parent string, job bool) []Node {
	id := "container/" + c.AgentID + "/" + c.ID
	nodes := []Node{{ID: id, Parent: parent, Kind: "container", Name: c.Name, State: s.ContainerState(c, job), Summary: c.Image, Relation: "observed placement", AgentID: c.AgentID, ContainerID: c.ID, Service: c.Service}}
	if c.Details != nil {
		for _, net := range c.Details.Networks {
			nodes = append(nodes, Node{ID: id + "/network/" + net.Name, Parent: id, Kind: "network", Name: net.Name, State: "observed", Summary: net.Address, Relation: "observed attachment", AgentID: c.AgentID})
		}
		for i, mount := range c.Details.Mounts {
			nodes = append(nodes, Node{ID: fmt.Sprintf("%s/mount/%d", id, i), Parent: id, Kind: "mount", Name: mount.Source + " → " + mount.Target, State: "observed", Relation: "observed mount", AgentID: c.AgentID})
		}
	}
	return nodes
}

func (s Snapshot) FleetTopology() []Node {
	root := "server/" + s.ServerName
	nodes := []Node{{ID: root, Kind: "server", Name: s.ServerName, State: "online", Summary: "URUFLOW " + s.Version, Relation: "control plane"},
		{ID: root + "/registry", Parent: root, Kind: "registry", Name: s.RegistryAddress, State: s.Registry, Relation: "artifact distribution"}}
	for _, a := range s.Agents {
		count := 0
		for _, c := range s.Containers {
			if c.AgentID == a.ID {
				count++
			}
		}
		roles := []string{}
		for _, r := range a.Roles {
			roles = append(roles, string(r))
		}
		nodes = append(nodes, Node{ID: "agent/" + a.ID, Parent: root, Kind: "agent", Name: a.Name, State: string(a.Status), AgentID: a.ID, Summary: fmt.Sprintf("%s / %d observed containers", strings.Join(roles, ","), count), Relation: "outbound authenticated UFP link"})
	}
	for _, p := range s.Projects {
		nodes = append(nodes, Node{ID: "project/" + p.Name, Kind: "project", Name: p.Name, State: s.ProjectState(p), Summary: fmt.Sprintf("%d services / %d runners", len(p.ServiceList()), len(p.Runners)), Relation: "project environment"})
	}
	for _, container := range s.Containers {
		parent := "agent/" + container.AgentID
		if s.Agent(container.AgentID) == nil {
			continue
		}
		job, configured := false, false
		if project := s.Project(container.Project); project != nil {
			for _, service := range project.ServiceList() {
				if service.Name != container.Service {
					continue
				}
				job = service.EffectiveMode() == models.ServiceModeJob
				for _, runner := range project.Runners {
					if runner == container.AgentID {
						configured = true
						break
					}
				}
				break
			}
		}
		observed := containerNodes(s, container, parent, job)
		if container.Project != "" && !configured {
			observed[0].Relation = "unexpected observation"
		}
		nodes = append(nodes, observed...)
	}

	return nodes
}

func (s Snapshot) ServiceState(p models.Project, service models.Service) string {
	p.Services = []models.Service{service}
	return s.ProjectState(p)
}
