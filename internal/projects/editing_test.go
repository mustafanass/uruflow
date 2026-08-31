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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPastedConfigAloneCreatesTheFiles(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	item := Draft{
		Project:    "web",
		Env:        "stg",
		Definition: Definition{Git: "git@github.com:acme/web.git"},
		RawYAML: "branch: staging\nbuilder: builder-01\nrunners: [web-01, web-02]\n" +
			"ports: [\"8090:80\"]\nauto_deploy: false\n",
		RawEnv: "MODE=staging\n",
	}

	if err := loader.Write(item); err != nil {
		t.Fatalf("write: %v", err)
	}

	result := loader.Load()
	if len(result.Problems) != 0 {
		t.Fatalf("problems: %v", result.Problems)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("loaded %d", len(result.Projects))
	}

	project := result.Projects[0]
	if project.Name != "web-stg" || project.Branch != "staging" {
		t.Fatalf("project = %q branch = %q", project.Name, project.Branch)
	}
	if len(project.Runners) != 2 {
		t.Fatalf("runners = %v", project.Runners)
	}
	if project.AutoDeploy {
		t.Error("auto_deploy from the pasted config was ignored")
	}
	if project.Runtime.Env["MODE"] != "staging" {
		t.Fatalf("variables = %v", project.Runtime.Env)
	}
}

func TestEditingPreservesKeysTheFormDoesNotOwn(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	definitionPath, environmentPath, _ := loader.Paths("api", "dev")
	os.MkdirAll(filepath.Dir(definitionPath), 0o755)
	os.WriteFile(definitionPath, []byte(
		"git: git@host:api.git\nbuild_args:\n  VERSION: \"2\"\nenv:\n  APP: api\n"), 0o644)
	os.WriteFile(environmentPath, []byte(
		"branch: develop\nbuilder: builder-01\nrunners: [dev-01]\ncommand: sh -c 'serve'\nenv:\n  TIER: edge\n"), 0o644)

	auto := false
	if err := loader.Write(Draft{
		Project:    "api",
		Env:        "dev",
		Definition: Definition{Git: "git@host:api.git", Dockerfile: "Dockerfile"},
		Environment: Environment{
			Branch: "main", Builder: "builder-01",
			Runners: []string{"web-01"}, AutoDeploy: &auto, Ports: []string{"80:80"},
		},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	definition, _ := os.ReadFile(definitionPath)
	if !strings.Contains(string(definition), "VERSION") {
		t.Errorf("build_args were lost:\n%s", definition)
	}
	if !strings.Contains(string(definition), "APP") {
		t.Errorf("project env was lost:\n%s", definition)
	}
	if strings.Contains(string(definition), `name: ""`) {
		t.Errorf("empty keys were written:\n%s", definition)
	}

	environment, _ := os.ReadFile(environmentPath)
	if !strings.Contains(string(environment), "serve") {
		t.Errorf("command was lost:\n%s", environment)
	}
	if !strings.Contains(string(environment), "TIER") {
		t.Errorf("environment env block was lost:\n%s", environment)
	}
	if !strings.Contains(string(environment), "branch: main") {
		t.Errorf("the form value did not win:\n%s", environment)
	}
	if !strings.Contains(string(environment), "web-01") {
		t.Errorf("runners were not replaced:\n%s", environment)
	}
}

func TestTyposAreRejectedWithTheFieldName(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	definitionPath, environmentPath, _ := loader.Paths("api", "dev")
	os.MkdirAll(filepath.Dir(definitionPath), 0o755)
	os.WriteFile(definitionPath, []byte("git: git@host:api.git\n"), 0o644)
	os.WriteFile(environmentPath, []byte(
		"brnach: main\nbuilder: builder-01\nrunners: [dev-01]\n"), 0o644)

	result := loader.Load()

	if len(result.Problems) == 0 {
		t.Fatal("a misspelled key was accepted")
	}
	if !strings.Contains(result.Problems[0].Error(), "brnach") {
		t.Fatalf("the problem does not name the bad key: %s", result.Problems[0].Error())
	}
}

func TestStructuredServicesWriteAndLoadBack(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())
	item := draft()
	retries := 4
	item.Environment.Services = map[string]Service{
		"api": {
			Dockerfile: "Dockerfile", Ports: []string{"8080:8080"},
			Healthcheck: &Healthcheck{Type: "http", Path: "/ready", Port: 8080, Interval: "2s", Timeout: "1s", Retries: &retries},
			Labels:      map[string]string{"traefik.enable": "true"},
		},
		"cache": {
			Image:       "redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Healthcheck: &Healthcheck{Type: "tcp", Port: 6379},
			Labels:      map[string]string{"metrics.enabled": "yes"},
		},
	}
	if err := loader.Write(item); err != nil {
		t.Fatal(err)
	}
	result := loader.Load()
	if len(result.Problems) != 0 || len(result.Projects) != 1 || len(result.Projects[0].Services) != 2 {
		t.Fatalf("result = %+v", result)
	}
	byName := map[string]Service{}
	environment, err := readEnvironment(result.Projects[0].Source)
	if err != nil {
		t.Fatal(err)
	}
	for name, service := range environment.Services {
		byName[name] = service
	}
	if byName["api"].Labels["traefik.enable"] != "true" || byName["api"].Healthcheck.Path != "/ready" || byName["cache"].Image == "" {
		t.Fatalf("services = %+v", byName)
	}
}
