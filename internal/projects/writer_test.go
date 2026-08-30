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

func draft() Draft {
	auto := true
	return Draft{
		Project: "api",
		Env:     "dev",
		Definition: Definition{
			Git:        "git@github.com:acme/api.git",
			Dockerfile: "Dockerfile",
			Context:    ".",
		},
		Environment: Environment{
			Branch:     "develop",
			Builder:    "builder-01",
			Runners:    []string{"dev-01"},
			AutoDeploy: &auto,
			Ports:      []string{"8081:80"},
		},
		RawEnv: "LOG_LEVEL=debug\nDATABASE_URL=postgres://dev\n",
	}
}

func TestWrittenFilesLoadBackIdentically(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	if err := loader.Write(draft()); err != nil {
		t.Fatalf("write: %v", err)
	}

	result := loader.Load()
	if len(result.Problems) != 0 {
		t.Fatalf("written files did not load cleanly: %v", result.Problems)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("loaded %d projects", len(result.Projects))
	}

	project := result.Projects[0]
	if project.Name != "api-dev" || project.Env != "dev" {
		t.Fatalf("project = %q env = %q", project.Name, project.Env)
	}
	if project.Branch != "develop" || !project.AutoDeploy {
		t.Fatalf("branch = %q auto = %v", project.Branch, project.AutoDeploy)
	}
	if len(project.Runtime.Ports) != 1 || project.Runtime.Ports[0].Host != 8081 {
		t.Fatalf("ports = %+v", project.Runtime.Ports)
	}
	if project.Runtime.Env["LOG_LEVEL"] != "debug" {
		t.Fatalf("env = %v", project.Runtime.Env)
	}
}

func TestRawYAMLIsWrittenVerbatim(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	pasted := "# pasted by hand\nbranch: main\nbuilder: builder-01\nrunners: [web-01]\nports: [\"80:80\"]\n"
	item := draft()
	item.RawYAML = pasted

	if err := loader.Write(item); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, environmentPath, _ := loader.Paths("api", "dev")
	written, _ := os.ReadFile(environmentPath)
	if string(written) != pasted {
		t.Fatalf("pasted yaml was rewritten:\n%s", written)
	}
	if !strings.Contains(string(written), "# pasted by hand") {
		t.Error("comments were lost")
	}

	result := loader.Load()
	if len(result.Problems) != 0 {
		t.Fatalf("pasted yaml did not load: %v", result.Problems)
	}
	if result.Projects[0].Branch != "main" {
		t.Fatalf("pasted branch was ignored: %q", result.Projects[0].Branch)
	}
}

func TestEmptyEnvRemovesTheFile(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	loader.Write(draft())
	_, _, variablesPath := loader.Paths("api", "dev")
	if _, err := os.Stat(variablesPath); err != nil {
		t.Fatalf("env file was not written: %v", err)
	}

	item := draft()
	item.RawEnv = "  \n"
	loader.Write(item)

	if _, err := os.Stat(variablesPath); !os.IsNotExist(err) {
		t.Error("clearing the env tab left the file behind")
	}
}

func TestWriteRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	for name, item := range map[string]Draft{
		"project escape": {Project: "../evil", Env: "dev"},
		"env escape":     {Project: "api", Env: "../../evil"},
		"slash":          {Project: "api/sub", Env: "dev"},
		"uppercase":      {Project: "API", Env: "dev"},
		"empty project":  {Project: "", Env: "dev"},
		"empty env":      {Project: "api", Env: ""},
	} {
		if err := loader.Write(item); err == nil {
			t.Errorf("%s: write was accepted", name)
		}
	}

	if entries, _ := os.ReadDir(filepath.Dir(root)); len(entries) == 0 {
		t.Skip("nothing to inspect")
	}
}

func TestRemoveDropsTheEnvironmentAndFolderWhenLast(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())

	loader.Write(draft())
	second := draft()
	second.Env = "prod"
	second.RawEnv = ""
	loader.Write(second)

	if err := loader.Remove("api", "dev"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	_, devPath, _ := loader.Paths("api", "dev")
	if _, err := os.Stat(devPath); !os.IsNotExist(err) {
		t.Error("dev.yaml survived removal")
	}
	definitionPath, _, _ := loader.Paths("api", "prod")
	if _, err := os.Stat(definitionPath); err != nil {
		t.Error("project.yaml was removed while another environment remained")
	}

	if err := loader.Remove("api", "prod"); err != nil {
		t.Fatalf("remove last: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(definitionPath)); !os.IsNotExist(err) {
		t.Error("the project folder survived removing its last environment")
	}
}

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
