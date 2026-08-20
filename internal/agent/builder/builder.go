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
	"path/filepath"
	"strings"

	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	shortCommitLength = 12
	gitBinary         = "git"
	dockerBinary      = "docker"
)

type Builder struct {
	workDir string
}

type Result struct {
	Image  string
	Images map[string]string
	Commit string
	Digest string
}

func New(workDir string) *Builder {
	os.MkdirAll(workDir, 0o755)
	return &Builder{workDir: workDir}
}

func (b *Builder) Build(ctx context.Context, request ufp.BuildRequest, log LogFunc) (*Result, error) {
	sourceDir := filepath.Join(b.workDir, request.Project)

	log(ufp.StreamStdout, fmt.Sprintf("fetching %s (%s)", request.GitURL, request.Branch))
	if err := b.sync(ctx, sourceDir, request, log); err != nil {
		return nil, err
	}

	commit, err := capture(ctx, sourceDir, gitBinary, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	result := &Result{Commit: commit, Images: make(map[string]string, len(request.Targets))}

	for _, target := range request.Targets {
		tags := targetTags(target, request.Tags, shortCommit(commit))

		label := target.Service
		if label == "" {
			label = request.Project
		}
		log(ufp.StreamStdout, fmt.Sprintf("building %s at %s", label, shortCommit(commit)))

		if err := b.build(ctx, sourceDir, target, tags, log); err != nil {
			return nil, err
		}

		for _, tag := range tags {
			log(ufp.StreamStdout, "pushing "+tag)
			if err := stream(ctx, sourceDir, log, dockerBinary, "push", tag); err != nil {
				return nil, err
			}
		}

		result.Images[target.Service] = tags[0]
		if result.Image == "" {
			result.Image = tags[0]
			digest, err := capture(ctx, sourceDir, dockerBinary,
				"image", "inspect", "--format", "{{index .RepoDigests 0}}", tags[0])
			if err == nil {
				result.Digest = parseDigest(digest)
			}
		}
	}

	return result, nil
}

func (b *Builder) sync(ctx context.Context, sourceDir string, request ufp.BuildRequest, log LogFunc) error {
	if _, err := os.Stat(filepath.Join(sourceDir, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(sourceDir), 0o755); err != nil {
			return err
		}
		os.RemoveAll(sourceDir)
		if err := stream(ctx, b.workDir, log, gitBinary,
			"clone", "--branch", request.Branch, request.GitURL, sourceDir); err != nil {
			return err
		}
	} else if err := stream(ctx, sourceDir, log, gitBinary, "fetch", "--prune", "origin"); err != nil {
		return err
	}

	target := request.Commit
	if target == "" {
		target = "origin/" + request.Branch
	}
	return stream(ctx, sourceDir, log, gitBinary, "reset", "--hard", target)
}

func (b *Builder) build(ctx context.Context, sourceDir string, target ufp.BuildTarget, tags []string, log LogFunc) error {
	args := []string{"build", "-f", filepath.Join(sourceDir, target.Dockerfile)}

	for _, tag := range tags {
		args = append(args, "-t", tag)
	}
	for key, value := range target.BuildArgs {
		args = append(args, "--build-arg", key+"="+value)
	}
	args = append(args, filepath.Join(sourceDir, target.Context))

	return stream(ctx, sourceDir, log, dockerBinary, args...)
}

func targetTags(target ufp.BuildTarget, extra []string, commit string) []string {
	tags := []string{target.Image + ":" + commit}
	for _, tag := range extra {
		tags = append(tags, target.Image+":"+tag)
	}
	return tags
}

func shortCommit(commit string) string {
	if len(commit) <= shortCommitLength {
		return commit
	}
	return commit[:shortCommitLength]
}

func parseDigest(reference string) string {
	_, digest, found := strings.Cut(reference, "@")
	if !found {
		return ""
	}
	return digest
}
