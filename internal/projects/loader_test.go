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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func fakeAgents() Resolver {
	agents := map[string]*models.Agent{
		"builder-01": {ID: "b1", Name: "builder-01", Roles: []models.Role{models.RoleBuilder}},
		"dev-01":     {ID: "d1", Name: "dev-01", Roles: []models.Role{models.RoleRunner}},
		"web-01":     {ID: "w1", Name: "web-01", Roles: []models.Role{models.RoleRunner}},
		"web-02":     {ID: "w2", Name: "web-02", Roles: []models.Role{models.RoleRunner}},
	}

	return func(name string) (*models.Agent, error) {
		agent, ok := agents[name]
		if !ok {
			return nil, fmt.Errorf("no agent %q", name)
		}
		return agent, nil
	}
}

func seedTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	write(t, filepath.Join(root, "defaults.yaml"), "env:\n  TZ: Asia/Baghdad\n  LOG_LEVEL: info\n")

	write(t, filepath.Join(root, "projects", "api", "dev.yaml"),
		"timeout: 90m\nbuilder: builder-01\nrunners: [dev-01]\nports: [\"8081:80\"]\n"+
			"env:\n  APP: api\n  LOG_LEVEL: warn\nservices:\n  api:\n    git: git@github.com:acme/api.git\n    branch: develop\n    dockerfile: Dockerfile\n    context: .\n")
	write(t, filepath.Join(root, "projects", "api", "dev.env"),
		"# comment\nLOG_LEVEL=debug\nDATABASE_URL=postgres://dev\n")

	write(t, filepath.Join(root, "projects", "api", "prod.yaml"),
		"builder: builder-01\nrunners: [web-01, web-02]\nports: [\"80:80\"]\n"+
			"volumes: [\"/srv/api:/data:ro\"]\nenv:\n  APP: api\n  LOG_LEVEL: warn\n  MODE: production\n"+
			"services:\n  api:\n    git: git@github.com:acme/api.git\n    branch: main\n    dockerfile: Dockerfile\n    context: .\n")

	return root
}

func TestLoaderExpandsEnvironmentsIntoProjects(t *testing.T) {
	result := NewLoader(seedTree(t), fakeAgents()).Load()

	if len(result.Problems) != 0 {
		t.Fatalf("unexpected problems: %v", result.Problems)
	}
	if len(result.Projects) != 2 {
		t.Fatalf("loaded %d projects, want 2", len(result.Projects))
	}

	dev, prod := result.Projects[0], result.Projects[1]
	if dev.Name != "api-dev" || prod.Name != "api-prod" {
		t.Fatalf("names = %q, %q", dev.Name, prod.Name)
	}
	if dev.Env != "dev" || dev.Base() != "api" {
		t.Fatalf("dev env = %q base = %q", dev.Env, dev.Base())
	}
	if !dev.Managed() || dev.Source == "" {
		t.Fatal("a file-backed project is not marked managed")
	}
	if dev.Timeout != 90*time.Minute || prod.EffectiveTimeout() != models.DefaultDeploymentTimeout {
		t.Fatalf("timeouts dev=%s prod=%s", dev.Timeout, prod.EffectiveTimeout())
	}

	if dev.Services[0].Branch != "develop" || prod.Services[0].Branch != "main" {
		t.Fatalf("service branches = %q, %q", dev.Services[0].Branch, prod.Services[0].Branch)
	}
	if dev.GitURL != "" || prod.GitURL != "" || dev.AutoDeploy || prod.AutoDeploy {
		t.Fatal("project-level source or auto deploy was populated")
	}
	if len(prod.Runners) != 2 || len(dev.Runners) != 1 {
		t.Fatalf("runners dev=%v prod=%v", dev.Runners, prod.Runners)
	}
	if dev.Runtime.Ports[0].Host != 8081 || prod.Runtime.Ports[0].Host != 80 {
		t.Fatalf("ports dev=%+v prod=%+v", dev.Runtime.Ports, prod.Runtime.Ports)
	}
	if len(prod.Runtime.Volumes) != 1 || !prod.Runtime.Volumes[0].ReadOnly {
		t.Fatalf("volumes = %+v", prod.Runtime.Volumes)
	}
}

func TestLoaderLoadsServiceOwnedSources(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "projects", "urufi", "prod.yaml"), `workflow: build_deploy
builder: builder-01
runners: [web-01]
services:
  core:
    git: git@host:urufi/core.git
    branch: main
    dockerfile: Dockerfile
  cache:
    image: redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
`)

	result := NewLoader(root, fakeAgents()).Load()
	if len(result.Problems) != 0 || len(result.Projects) != 1 {
		t.Fatalf("result = %+v", result)
	}
	project := result.Projects[0]
	if project.Name != "urufi-prod" || project.GitURL != "" || project.Branch != "" || project.AutoDeploy {
		t.Fatalf("project = %+v", project)
	}
	services := make(map[string]models.Service, len(project.Services))
	for _, service := range project.Services {
		services[service.Name] = service
	}
	if len(services) != 2 || services["core"].GitURL != "git@host:urufi/core.git" || services["cache"].Image == "" {
		t.Fatalf("services = %+v", project.Services)
	}
}

func TestEnvironmentRejectsAutoDeploy(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "projects", "api", "prod.yaml"), `builder: builder-01
runners: [web-01]
auto_deploy: true
services:
  api:
    git: git@host:api.git
    branch: main
    dockerfile: Dockerfile
`)

	result := NewLoader(root, fakeAgents()).Load()
	if len(result.Projects) != 0 || len(result.Problems) != 1 || !strings.Contains(result.Problems[0].Error(), "field auto_deploy not found") {
		t.Fatalf("result = %+v", result)
	}
}

func TestEnvironmentRejectsProjectLevelSourceFields(t *testing.T) {
	for _, field := range []string{"git", "branch", "dockerfile", "context", "build_args"} {
		t.Run(field, func(t *testing.T) {
			content := field + ": value\nworkflow: deploy_only\nrunners: [web-01]\nservices:\n  cache:\n    image: redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"
			err := ValidateEnvironmentYAML(content)
			if err == nil || !strings.Contains(err.Error(), "field "+field+" not found") {
				t.Fatalf("project-level field error = %v", err)
			}
		})
	}
}

func TestEnvPrecedenceDefaultsThenYAMLThenDotEnv(t *testing.T) {
	result := NewLoader(seedTree(t), fakeAgents()).Load()

	byName := map[string]models.Project{}
	for _, project := range result.Projects {
		byName[project.Name] = project
	}

	dev := byName["api-dev"].Runtime.Env
	if dev["TZ"] != "Asia/Baghdad" {
		t.Errorf("defaults.yaml did not reach the project: %v", dev)
	}
	if dev["APP"] != "api" {
		t.Errorf("environment YAML did not reach the project: %v", dev)
	}
	if dev["LOG_LEVEL"] != "debug" {
		t.Errorf("dev.env should win over environment YAML and defaults, got %q", dev["LOG_LEVEL"])
	}
	if dev["DATABASE_URL"] != "postgres://dev" {
		t.Errorf("dotenv value missing: %v", dev)
	}

	prod := byName["api-prod"].Runtime.Env
	if prod["LOG_LEVEL"] != "warn" {
		t.Errorf("without a .env, environment YAML should win over defaults, got %q", prod["LOG_LEVEL"])
	}
	if prod["MODE"] != "production" {
		t.Errorf("environment yaml env missing: %v", prod)
	}
}

func TestLoaderReportsBadFilesWithoutLosingGoodOnes(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "api", "broken.yaml"),
		"builder: nobody\nrunners: [web-01]\nservices:\n  api:\n    git: git@host:api.git\n    branch: main\n")
	if err := os.MkdirAll(filepath.Join(root, "projects", "web"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := NewLoader(root, fakeAgents()).Load()

	if len(result.Projects) != 2 {
		t.Fatalf("good projects were lost: %d loaded", len(result.Projects))
	}
	if len(result.Problems) != 2 {
		t.Fatalf("problems = %v", result.Problems)
	}

	joined := result.Problems[0].Error() + result.Problems[1].Error()
	if !strings.Contains(joined, "unknown agent") || !strings.Contains(joined, "no environment files found") {
		t.Fatalf("problems did not explain themselves: %v", result.Problems)
	}
}

func TestLoaderRejectsWrongRole(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "api", "bad.yaml"),
		"builder: web-01\nrunners: [web-01]\nservices:\n  api:\n    git: git@host:api.git\n    branch: main\n")

	result := NewLoader(root, fakeAgents()).Load()

	found := false
	for _, problem := range result.Problems {
		if strings.Contains(problem.Error(), "does not carry the builder role") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a runner used as builder was accepted: %v", result.Problems)
	}
}

func TestLoaderHandlesMissingDirectory(t *testing.T) {
	result := NewLoader(t.TempDir(), fakeAgents()).Load()

	if len(result.Projects) != 0 || len(result.Problems) != 0 {
		t.Fatalf("an empty config dir should be silent: %+v", result)
	}
}
