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

package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafanass/uruflow/internal/ufp"
)

func sourceCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func writeSource(t *testing.T, dir, name, value string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}

func commitSource(t *testing.T, dir string) string {
	t.Helper()
	sourceCommand(t, dir, "add", ".")
	sourceCommand(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "Update source")
	return sourceCommand(t, dir, "rev-parse", "HEAD")
}

func TestSelectiveBuildsReuseIndependentServicesAndTrackSharedInputs(t *testing.T) {
	source, work, bin := t.TempDir(), t.TempDir(), t.TempDir()
	sourceCommand(t, source, "init", "--initial-branch=main")
	names := []string{"rust", "console", "core"}
	for _, name := range names {
		writeSource(t, source, name+"/Dockerfile", "FROM scratch\nCOPY . /src\n")
		writeSource(t, source, name+"/source", "original")
	}
	writeSource(t, source, "shared/schema", "original")
	initialCommit := commitSource(t, source)
	trace := filepath.Join(t.TempDir(), "commands")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$URUFLOW_TEST_COMMANDS"
case "$1" in
  pull) exit 1 ;;
  image)
    printf '["%s@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]\n' "${5%:*}" ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("URUFLOW_TEST_COMMANDS", trace)
	request := ufp.BuildRequest{Project: "multi", Tags: []string{"latest"}}
	for _, name := range names {
		request.Targets = append(request.Targets, ufp.BuildTarget{Service: name, Image: "registry/" + name, GitURL: source, Branch: "main", Dockerfile: name + "/Dockerfile", Context: ".", Watch: []string{name, "shared"}})
	}
	builder := New(work)
	var logs []string
	log := func(_, line string) { logs = append(logs, line) }
	build := func(previous *Result) *Result {
		t.Helper()
		if previous != nil {
			for i := range request.Targets {
				target := &request.Targets[i]
				target.PreviousImage, target.PreviousCommit = previous.Images[target.Service], previous.Commits[target.Service]
			}
		}
		result, err := builder.Build(context.Background(), request, log)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Images) != 3 || len(result.Commits) != 3 {
			t.Fatalf("incomplete release: %+v", result)
		}
		return result
	}
	count := func() int {
		t.Helper()
		data, err := os.ReadFile(trace)
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "build ") {
				total++
			}
		}
		return total
	}
	first := build(nil)
	if count() != 3 {
		t.Fatal("first release did not build all services")
	}
	writeSource(t, source, "console/source", "updated")
	writeSource(t, source, "core/source", "updated")
	secondCommit := commitSource(t, source)
	second := build(first)
	if count() != 5 || second.Commit != secondCommit || second.Commits["rust"] != initialCommit || second.Commits["console"] != secondCommit || second.Images["rust"] != first.Images["rust"] {
		t.Fatalf("unchanged Rust service was not reused: builds=%d result=%+v", count(), second)
	}
	writeSource(t, source, "shared/schema", "updated")
	sharedCommit := commitSource(t, source)
	third := build(second)
	if count() != 8 || third.Commits["rust"] != sharedCommit {
		t.Fatal("shared dependency did not rebuild all affected services")
	}
	writeSource(t, source, "README.md", "documentation only")
	latestCommit := commitSource(t, source)
	fourth := build(third)
	if count() != 8 || fourth.Commit != latestCommit || fourth.Commits["rust"] != sharedCommit {
		t.Fatal("reuse-only release lost its current source revision or original build commit")
	}
	request.Rebuild = true
	build(fourth)
	if count() != 11 {
		t.Fatal("forced rebuild reused an image")
	}
	data, _ := os.ReadFile(trace)
	if strings.Count(string(data), "--pull --no-cache") != 3 {
		t.Fatal("forced rebuild did not bypass Docker cache")
	}
	if strings.Contains("\n"+string(data), "\npull ") {
		t.Fatal("builder downloaded an image to check reuse")
	}
	if !strings.Contains(strings.Join(logs, "\n"), "reusing rust") {
		t.Fatal("reused image is missing from logs")
	}
}

func TestUnchangedSourceIncludesDockerfileIgnoreFilesAndDirtyInputs(t *testing.T) {
	dir := t.TempDir()
	sourceCommand(t, dir, "init", "--initial-branch=main")
	for name, value := range map[string]string{"app/Dockerfile": "FROM scratch", "app/source": "first", "shared/schema": "first", ".dockerignore": ".git"} {
		writeSource(t, dir, name, value)
	}
	original := commitSource(t, dir)
	target := ufp.BuildTarget{Dockerfile: "app/Dockerfile", Context: ".", Watch: []string{"app", "shared"}, PreviousImage: "registry/app@sha256:" + strings.Repeat("a", 64), PreviousCommit: original}
	if !unchangedSource(context.Background(), dir, target, original) {
		t.Fatal("clean unchanged source was not reusable")
	}
	for _, path := range []string{"app/Dockerfile", ".dockerignore", "app/Dockerfile.dockerignore", "shared/schema"} {
		t.Run(path, func(t *testing.T) {
			sourceCommand(t, dir, "reset", "--hard", original)
			writeSource(t, dir, path, fmt.Sprintf("changed %s", path))
			if unchangedSource(context.Background(), dir, target, original) {
				t.Fatal("dirty build input was reused")
			}
			current := commitSource(t, dir)
			if unchangedSource(context.Background(), dir, target, current) {
				t.Fatal("changed build input was reused")
			}
		})
	}
	sourceCommand(t, dir, "reset", "--hard", original)
	target.PreviousCommit = strings.Repeat("b", 40)
	if unchangedSource(context.Background(), dir, target, original) {
		t.Fatal("unknown base commit was reused")
	}
	target.PreviousCommit = original
	target.Watch = nil
	writeSource(t, dir, "other/source", "changed")
	current := commitSource(t, dir)
	if unchangedSource(context.Background(), dir, target, current) {
		t.Fatal("default build context ignored a source change")
	}
}

func TestWatchedSymlinksUseTheFullContextForComparison(t *testing.T) {
	dir := t.TempDir()
	sourceCommand(t, dir, "init", "--initial-branch=main")
	writeSource(t, dir, "app/Dockerfile", "FROM scratch")
	writeSource(t, dir, "shared/source", "initial")
	if err := os.Symlink("../shared/source", filepath.Join(dir, "app", "linked")); err != nil {
		t.Skip(err)
	}
	previous := commitSource(t, dir)
	writeSource(t, dir, "shared/source", "updated")
	current := commitSource(t, dir)
	target := ufp.BuildTarget{Context: ".", Dockerfile: "app/Dockerfile", Watch: []string{"app"}, PreviousImage: "registry/app@sha256:" + strings.Repeat("a", 64), PreviousCommit: previous}
	if unchangedSource(context.Background(), dir, target, current) {
		t.Fatal("a symlink hid a changed source input")
	}
}
