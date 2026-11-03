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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mustafanass/uruflow/internal/ufp"
)

func unchangedSource(ctx context.Context, sourceDir string, target ufp.BuildTarget, commit string) bool {
	if target.PreviousImage == "" || target.PreviousCommit == "" {
		return false
	}
	paths := append([]string(nil), target.Watch...)
	if len(paths) == 0 {
		paths = append(paths, target.Context)
	}
	paths = append(paths, target.Dockerfile, target.Dockerfile+".dockerignore", filepath.Join(target.Context, ".dockerignore"))
	for _, path := range []string{target.Context, target.Dockerfile} {
		resolved, err := safeBuildPath(sourceDir, path)
		if err != nil {
			return false
		}
		relative, err := filepath.Rel(sourceDir, resolved)
		if err != nil {
			return false
		}
		if path == target.Dockerfile || len(target.Watch) == 0 {
			paths = append(paths, relative)
		}
	}
	if len(target.Watch) > 0 {
		args := append([]string{"--literal-pathspecs", "ls-files", "--stage", "--"}, paths...)
		entries, err := capture(ctx, sourceDir, gitBinary, args...)
		if err != nil {
			return false
		}
		for _, entry := range strings.Split(entries, "\n") {
			if strings.HasPrefix(entry, "120000 ") {
				paths = append(paths, target.Context)
				break
			}
		}
	}
	args := append([]string{"--literal-pathspecs", "diff", "--quiet", target.PreviousCommit, commit, "--"}, paths...)
	command := exec.CommandContext(ctx, gitBinary, args...)
	command.Dir = sourceDir
	if err := command.Run(); err != nil {
		return false
	}
	args = append([]string{"--literal-pathspecs", "status", "--porcelain", "--untracked-files=normal", "--ignored", "--"}, paths...)
	dirty, err := capture(ctx, sourceDir, gitBinary, args...)
	return err == nil && dirty == ""
}

func (r *Result) record(service, image, commit string) {
	r.Images[service] = image
	r.Commits[service] = commit
	if r.Image == "" {
		r.Image, r.Digest = image, parseDigest(image)
	}
}
