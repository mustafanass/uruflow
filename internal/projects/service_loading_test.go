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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func TestServicesLoadFromTheEnvironmentFile(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "shop", "prod.yaml"),
		"builder: builder-01\nrunners: [web-01]\n"+
			"env:\n  APP: shop\n  SHARED: yes\n"+
			"services:\n"+
			"  app:\n    git: git@github.com:acme/shop.git\n    branch: main\n    dockerfile: Dockerfile\n    context: .\n    ports: [\"8080:80\"]\n"+
			"    env:\n      ROLE: web\n"+
			"  worker:\n    git: git@github.com:acme/shop.git\n    branch: main\n    dockerfile: Dockerfile.worker\n    command: ./worker\n"+
			"  cache:\n    image: redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n    volumes: [\"/srv/shop/redis:/data\"]\n")

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
	if byName["cache"].Built() || byName["cache"].Image != "redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
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

func TestNativeResourcesAndInterpolationLoadAsOneProjectModel(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "native", "prod.env"),
		"NETWORK_NAME=native-edge\nHOST_IP=127.0.0.1\nDB_ALIAS=postgres\nDB_IMAGE=postgres@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc\n")
	write(t, filepath.Join(root, "projects", "native", "prod.yaml"),
		"builder: builder-01\nrunners: [web-01]\n"+
			"resources:\n  networks:\n    data:\n      name: ${NETWORK_NAME:?network required}\n      internal: true\n  volumes:\n    state: {}\n"+
			"services:\n  database:\n    image: ${DB_IMAGE:?image required}\n    ports: [\"${HOST_IP}:5432:5432\"]\n    networks:\n      data:\n        aliases: [\"${DB_ALIAS:-database}\"]\n    mounts:\n      - type: volume\n        source: state\n        target: /var/lib/postgresql/data\n    healthcheck:\n      type: command\n      command: [pg_isready]\n      interval: 1s\n      timeout: 1s\n      retries: 3\n  api:\n    git: git@host:native.git\n    branch: main\n    dockerfile: Dockerfile\n    networks:\n      data: {}\n    depends_on:\n      database: healthy\n")

	result := NewLoader(root, fakeAgents()).Load()
	if len(result.Problems) != 0 {
		t.Fatalf("problems: %v", result.Problems)
	}
	var native *models.Project
	for index := range result.Projects {
		if result.Projects[index].Name == "native-prod" {
			native = &result.Projects[index]
		}
	}
	if native == nil {
		t.Fatal("native-prod was not loaded")
	}
	if native.Networks["data"].Name != "native-edge" || !native.Networks["data"].Internal || native.Volumes["state"].Name != "native-prod-state" {
		t.Fatalf("resources = networks:%#v volumes:%#v", native.Networks, native.Volumes)
	}
	byName := map[string]models.Service{}
	for _, service := range native.Services {
		byName[service.Name] = service
	}
	if byName["database"].Ports[0].HostIP != "127.0.0.1" || byName["database"].Networks[0].Aliases[0] != "postgres" || byName["database"].Volumes[0].Type != "volume" || byName["database"].Healthcheck.Type != "command" {
		t.Fatalf("database = %#v", byName["database"])
	}
	ordered, err := models.OrderServices(native.Services)
	if err != nil || ordered[0].Name != "database" || ordered[1].Name != "api" {
		t.Fatalf("dependency order = %#v, %v", ordered, err)
	}
}

func TestServiceRejectsImageAndDockerfileTogether(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "bad", "prod.yaml"),
		"builder: builder-01\nrunners: [web-01]\n"+
			"services:\n  app:\n    git: git@host:bad.git\n    branch: main\n    image: redis:7\n    dockerfile: Dockerfile\n")

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

func TestSingleServiceProjectsRemainExplicit(t *testing.T) {
	result := NewLoader(seedTree(t), fakeAgents()).Load()

	for _, project := range result.Projects {
		if !project.MultiService() {
			t.Fatalf("%s did not retain its explicit service", project.Name)
		}
		list := project.ServiceList()
		if len(list) != 1 || list[0].Name != "api" {
			t.Fatalf("%s services = %+v", project.Name, list)
		}
	}
}

func TestServicesLoadHealthchecksAndLabels(t *testing.T) {
	root := seedTree(t)
	write(t, filepath.Join(root, "projects", "checks", "prod.yaml"),
		"builder: builder-01\nrunners: [web-01]\nservices:\n"+
			"  api:\n    git: git@host:checks.git\n    branch: main\n    dockerfile: Dockerfile\n    healthcheck:\n      type: http\n      path: /health\n      port: 8080\n      interval: 2s\n      timeout: 1s\n      retries: 4\n"+
			"    labels:\n      traefik.enable: \"true\"\n      monitor.team: platform\n"+
			"  cache:\n    image: redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n    healthcheck:\n      type: tcp\n      port: 6379\n"+
			"    labels:\n      metrics.enabled: \"yes\"\n"+
			"  worker:\n    git: git@host:checks.git\n    branch: main\n    dockerfile: Dockerfile.worker\n    healthcheck:\n      type: running\n      stable_for: 8s\n")

	result := NewLoader(root, fakeAgents()).Load()
	if len(result.Problems) != 0 {
		t.Fatalf("problems: %v", result.Problems)
	}
	var project models.Project
	for index := range result.Projects {
		if result.Projects[index].Name == "checks-prod" {
			project = result.Projects[index]
		}
	}
	services := map[string]models.Service{}
	for _, service := range project.Services {
		services[service.Name] = service
	}
	api := services["api"]
	if api.Healthcheck == nil || api.Healthcheck.Type != "http" || api.Healthcheck.Scheme != "http" || api.Healthcheck.Interval != 2*time.Second || api.Healthcheck.Retries != 4 {
		t.Fatalf("api healthcheck = %+v", api.Healthcheck)
	}
	if api.Labels["traefik.enable"] != "true" || api.Labels["monitor.team"] != "platform" {
		t.Fatalf("api labels = %#v", api.Labels)
	}
	if cache := services["cache"]; cache.Built() || cache.Healthcheck.Timeout != 3*time.Second || cache.Labels["metrics.enabled"] != "yes" {
		t.Fatalf("cache = %+v", cache)
	}
	if worker := services["worker"]; worker.Healthcheck.StableFor != 8*time.Second {
		t.Fatalf("worker healthcheck = %+v", worker.Healthcheck)
	}
}

func TestInvalidHealthchecksAndReservedLabelsAreRejected(t *testing.T) {
	cases := map[string]string{
		"unknown type":           "healthcheck:\n      type: exec\n",
		"missing path":           "healthcheck:\n      type: http\n      port: 8080\n",
		"bad port":               "healthcheck:\n      type: tcp\n      port: 70000\n",
		"zero duration":          "healthcheck:\n      type: tcp\n      port: 80\n      timeout: 0s\n",
		"zero retries":           "healthcheck:\n      type: tcp\n      port: 80\n      retries: 0\n",
		"bad path":               "healthcheck:\n      type: http\n      port: 80\n      path: health\n",
		"running missing stable": "healthcheck:\n      type: running\n",
		"reserved label":         "labels:\n      uruflow.project: forged\n",
	}
	for name, serviceConfig := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "projects", "bad", "prod.yaml"),
				"builder: builder-01\nrunners: [web-01]\nservices:\n  api:\n    git: git@host:bad.git\n    branch: main\n    dockerfile: Dockerfile\n    "+serviceConfig)
			result := NewLoader(root, fakeAgents()).Load()
			if len(result.Problems) == 0 {
				t.Fatal("invalid service configuration was accepted")
			}
			if !strings.Contains(result.Problems[0].Error(), "service") || !strings.Contains(result.Problems[0].Error(), "api") {
				t.Fatalf("error lacks service path: %v", result.Problems[0])
			}
		})
	}
}

func TestUnknownHealthcheckKeyIsRejected(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "projects", "bad", "prod.yaml"),
		"builder: builder-01\nrunners: [web-01]\nservices:\n  api:\n    git: git@host:bad.git\n    branch: main\n    healthcheck:\n      type: tcp\n      port: 80\n      intervaal: 2s\n")
	result := NewLoader(root, fakeAgents()).Load()
	if len(result.Problems) == 0 || !strings.Contains(result.Problems[0].Error(), "intervaal") {
		t.Fatalf("unknown healthcheck key was not reported: %v", result.Problems)
	}
}
