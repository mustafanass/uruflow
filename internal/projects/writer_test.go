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

func TestCreatePublishesProjectAndEnvironmentTogether(t *testing.T) {
	root := t.TempDir()
	loader := NewLoader(root, fakeAgents())
	item := draft()
	item.Environment.Services = map[string]Service{
		"api": {Dockerfile: "Dockerfile", Context: ".", Ports: []string{"8080:8080"}},
	}
	if err := loader.Create(item); err != nil {
		t.Fatalf("create: %v", err)
	}
	definitionPath, environmentPath, _ := loader.Paths("api", "dev")
	for _, path := range []string{definitionPath, environmentPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("created file %s: %v", path, err)
		}
	}
	if err := loader.Create(item); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate create error = %v", err)
	}
	result := loader.Load()
	if len(result.Problems) != 0 || len(result.Projects) != 1 {
		t.Fatalf("created project did not load: %+v", result)
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
