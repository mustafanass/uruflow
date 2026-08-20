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

	write(t, filepath.Join(root, "projects", "api", "project.yaml"),
		"git: git@github.com:acme/api.git\ndockerfile: Dockerfile\ncontext: .\nenv:\n  APP: api\n  LOG_LEVEL: warn\n")

	write(t, filepath.Join(root, "projects", "api", "dev.yaml"),
		"branch: develop\nbuilder: builder-01\nrunners: [dev-01]\nports: [\"8081:80\"]\nauto_deploy: true\n")
	write(t, filepath.Join(root, "projects", "api", "dev.env"),
		"# comment\nLOG_LEVEL=debug\nDATABASE_URL=postgres://dev\n")

	write(t, filepath.Join(root, "projects", "api", "prod.yaml"),
		"branch: main\nbuilder: builder-01\nrunners: [web-01, web-02]\nports: [\"80:80\"]\n"+
			"volumes: [\"/srv/api:/data:ro\"]\nauto_deploy: false\nenv:\n  MODE: production\n")

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

	if dev.Branch != "develop" || prod.Branch != "main" {
		t.Fatalf("branches = %q, %q", dev.Branch, prod.Branch)
	}
	if dev.GitURL != prod.GitURL {
		t.Fatal("both environments should inherit one git url")
	}
	if dev.AutoDeploy == prod.AutoDeploy {
		t.Fatal("auto_deploy should differ per environment")
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

func TestEnvPrecedenceDefaultsThenProjectThenYamlThenDotEnv(t *testing.T) {
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
		t.Errorf("project.yaml env did not reach the project: %v", dev)
	}
	if dev["LOG_LEVEL"] != "debug" {
		t.Errorf("dev.env should win over project.yaml and defaults, got %q", dev["LOG_LEVEL"])
	}
	if dev["DATABASE_URL"] != "postgres://dev" {
		t.Errorf("dotenv value missing: %v", dev)
	}

	prod := byName["api-prod"].Runtime.Env
	if prod["LOG_LEVEL"] != "warn" {
		t.Errorf("without a .env, project.yaml should win over defaults, got %q", prod["LOG_LEVEL"])
	}
	if prod["MODE"] != "production" {
		t.Errorf("environment yaml env missing: %v", prod)
	}
}

func TestLoaderReportsBadFilesWithoutLosingGoodOnes(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "api", "broken.yaml"),
		"branch: main\nbuilder: nobody\nrunners: [web-01]\n")
	write(t, filepath.Join(root, "projects", "web", "project.yaml"), "dockerfile: Dockerfile\n")

	result := NewLoader(root, fakeAgents()).Load()

	if len(result.Projects) != 2 {
		t.Fatalf("good projects were lost: %d loaded", len(result.Projects))
	}
	if len(result.Problems) != 2 {
		t.Fatalf("problems = %v", result.Problems)
	}

	joined := result.Problems[0].Error() + result.Problems[1].Error()
	if !strings.Contains(joined, "unknown agent") || !strings.Contains(joined, "git is required") {
		t.Fatalf("problems did not explain themselves: %v", result.Problems)
	}
}

func TestLoaderRejectsWrongRole(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "api", "bad.yaml"),
		"branch: main\nbuilder: web-01\nrunners: [web-01]\n")

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

func TestServicesLoadFromTheEnvironmentFile(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "shop", "project.yaml"),
		"git: git@github.com:acme/shop.git\nenv:\n  APP: shop\n")
	write(t, filepath.Join(root, "projects", "shop", "prod.yaml"),
		"branch: main\nbuilder: builder-01\nrunners: [web-01]\n"+
			"env:\n  SHARED: yes\n"+
			"services:\n"+
			"  app:\n    dockerfile: Dockerfile\n    context: .\n    ports: [\"8080:80\"]\n"+
			"    env:\n      ROLE: web\n"+
			"  worker:\n    dockerfile: Dockerfile.worker\n    command: ./worker\n"+
			"  cache:\n    image: redis:7-alpine\n    volumes: [\"/srv/shop/redis:/data\"]\n")

	result := NewLoader(root, fakeAgents()).Load()
	if len(result.Problems) != 0 {
		t.Fatalf("problems: %v", result.Problems)
	}

	var shop *models.Project
	for index := range result.Projects {
		if result.Projects[index].Name == "shop-prod" {
			shop = &result.Projects[index]
		}
	}
	if shop == nil {
		t.Fatal("shop-prod was not loaded")
	}

	if !shop.MultiService() || len(shop.Services) != 3 {
		t.Fatalf("services = %+v", shop.Services)
	}

	byName := map[string]models.Service{}
	for _, service := range shop.Services {
		byName[service.Name] = service
	}

	if !byName["app"].Built() || byName["app"].Ports[0].Host != 8080 {
		t.Fatalf("app = %+v", byName["app"])
	}
	if byName["worker"].Command != "./worker" || byName["worker"].BuildFile() != "Dockerfile.worker" {
		t.Fatalf("worker = %+v", byName["worker"])
	}
	if byName["cache"].Built() || byName["cache"].Image != "redis:7-alpine" {
		t.Fatalf("cache should be prebuilt: %+v", byName["cache"])
	}
	if len(byName["cache"].Volumes) != 1 {
		t.Fatalf("cache volumes = %+v", byName["cache"].Volumes)
	}

	env := shop.ServiceEnv(byName["app"])
	if env["APP"] != "shop" || env["SHARED"] != "yes" || env["ROLE"] != "web" {
		t.Fatalf("merged service env = %v", env)
	}
	if shop.ServiceEnv(byName["worker"])["ROLE"] != "" {
		t.Error("service env leaked between services")
	}
}

func TestServiceRejectsImageAndDockerfileTogether(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "bad", "project.yaml"), "git: git@host:bad.git\n")
	write(t, filepath.Join(root, "projects", "bad", "prod.yaml"),
		"branch: main\nbuilder: builder-01\nrunners: [web-01]\n"+
			"services:\n  app:\n    image: redis:7\n    dockerfile: Dockerfile\n")

	result := NewLoader(root, fakeAgents()).Load()

	found := false
	for _, problem := range result.Problems {
		if strings.Contains(problem.Error(), "both image and dockerfile") {
			found = true
		}
	}
	if !found {
		t.Fatalf("an ambiguous service was accepted: %v", result.Problems)
	}
}

func TestSingleServiceProjectsStayFlat(t *testing.T) {
	result := NewLoader(seedTree(t), fakeAgents()).Load()

	for _, project := range result.Projects {
		if project.MultiService() {
			t.Fatalf("%s became multi-service unexpectedly", project.Name)
		}
		list := project.ServiceList()
		if len(list) != 1 || list[0].Name != "" {
			t.Fatalf("%s implicit service = %+v", project.Name, list)
		}
	}
}
