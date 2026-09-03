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
		Project: "web",
		Env:     "stg",
		RawYAML: "builder: builder-01\nrunners: [web-01, web-02]\n" +
			"services:\n  web:\n    git: git@github.com:acme/web.git\n    branch: staging\n    dockerfile: Dockerfile\n    ports: [\"8090:80\"]\n",
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
	if project.Name != "web-stg" || project.Services[0].Branch != "staging" {
		t.Fatalf("project = %q services = %+v", project.Name, project.Services)
	}
	if len(project.Runners) != 2 {
		t.Fatalf("runners = %v", project.Runners)
	}
	if project.Runtime.Env["MODE"] != "staging" {
		t.Fatalf("variables = %v", project.Runtime.Env)
	}
}

func TestEditingPreservesKeysTheFormDoesNotOwn(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	environmentPath, _ := loader.Paths("api", "dev")
	os.MkdirAll(filepath.Dir(environmentPath), 0o755)
	os.WriteFile(environmentPath, []byte(
		"builder: builder-01\nrunners: [dev-01]\ncommand: sh -c 'serve'\nenv:\n  TIER: edge\n"+
			"services:\n  api:\n    git: git@host:api.git\n    branch: develop\n    dockerfile: Dockerfile\n    build_args:\n      VERSION: \"2\"\n"), 0o644)

	if err := loader.Write(Draft{
		Project: "api",
		Env:     "dev",
		Environment: Environment{
			Builder: "builder-01", Runners: []string{"web-01"}, Ports: []string{"80:80"},
		},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	environment, _ := os.ReadFile(environmentPath)
	if !strings.Contains(string(environment), "VERSION") {
		t.Errorf("service build_args were lost:\n%s", environment)
	}
	if !strings.Contains(string(environment), "serve") {
		t.Errorf("command was lost:\n%s", environment)
	}
	if !strings.Contains(string(environment), "TIER") {
		t.Errorf("environment env block was lost:\n%s", environment)
	}
	if !strings.Contains(string(environment), "web-01") {
		t.Errorf("runners were not replaced:\n%s", environment)
	}
}

func TestTyposAreRejectedWithTheFieldName(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	environmentPath, _ := loader.Paths("api", "dev")
	os.MkdirAll(filepath.Dir(environmentPath), 0o755)
	os.WriteFile(environmentPath, []byte(
		"brnach: main\nbuilder: builder-01\nrunners: [dev-01]\nservices:\n  api:\n    git: git@host:api.git\n    branch: main\n"), 0o644)

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
			Git: "git@host:api.git", Branch: "main", Dockerfile: "Dockerfile", Ports: []string{"8080:8080"},
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
