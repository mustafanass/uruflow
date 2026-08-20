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
	"encoding/json"
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
	if request.Project == "" || filepath.Base(request.Project) != request.Project || request.Project == "." {
		return nil, fmt.Errorf("invalid project name %q", request.Project)
	}
	sourceDir := filepath.Join(b.workDir, request.Project)

	log(ufp.StreamStdout, fmt.Sprintf("fetching source (%s)", request.Branch))
	if err := b.sync(ctx, sourceDir, request, log); err != nil {
		return nil, err
	}

	commit, err := capture(ctx, sourceDir, gitBinary, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	result := &Result{Commit: commit, Images: make(map[string]string, len(request.Targets))}

	for _, target := range request.Targets {
		if _, err := safeBuildPath(sourceDir, target.Dockerfile); err != nil {
			return nil, fmt.Errorf("dockerfile for %s: %w", target.Service, err)
		}
		if _, err := safeBuildPath(sourceDir, target.Context); err != nil {
			return nil, fmt.Errorf("context for %s: %w", target.Service, err)
		}
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

		digestRef, err := b.digestReference(ctx, sourceDir, target.Image, tags[0])
		if err != nil {
			return nil, err
		}
		result.Images[target.Service] = digestRef
		if result.Image == "" {
			result.Image = digestRef
			result.Digest = parseDigest(digestRef)
		}
	}

	return result, nil
}

func (b *Builder) digestReference(ctx context.Context, sourceDir, repository, tagged string) (string, error) {
	output, err := capture(ctx, sourceDir, dockerBinary,
		"image", "inspect", "--format", "{{json .RepoDigests}}", tagged)
	if err != nil {
		return "", err
	}

	var references []string
	if err := json.Unmarshal([]byte(output), &references); err != nil {
		return "", fmt.Errorf("read digest for %s: %w", tagged, err)
	}
	prefix := repository + "@sha256:"
	for _, reference := range references {
		if strings.HasPrefix(reference, prefix) {
			return reference, nil
		}
	}
	return "", fmt.Errorf("registry returned no digest for %s", repository)
}

func (b *Builder) sync(ctx context.Context, sourceDir string, request ufp.BuildRequest, log LogFunc) error {
	if _, err := os.Stat(filepath.Join(sourceDir, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(sourceDir), 0o755); err != nil {
			return err
		}
		if err := os.RemoveAll(sourceDir); err != nil {
			return err
		}
		if err := stream(ctx, b.workDir, log, gitBinary,
			"clone", "--branch", request.Branch, "--", request.GitURL, sourceDir); err != nil {
			return err
		}
	} else {
		if err != nil {
			return err
		}
		if err := stream(ctx, sourceDir, log, gitBinary, "fetch", "--prune", "origin"); err != nil {
			return err
		}
	}

	target := request.Commit
	if target == "" {
		target = "origin/" + request.Branch
	}
	if strings.HasPrefix(target, "-") {
		return fmt.Errorf("invalid git target %q", target)
	}
	return stream(ctx, sourceDir, log, gitBinary, "reset", "--hard", target)
}

func (b *Builder) build(ctx context.Context, sourceDir string, target ufp.BuildTarget, tags []string, log LogFunc) error {
	dockerfile, err := safeBuildPath(sourceDir, target.Dockerfile)
	if err != nil {
		return err
	}
	buildContext, err := safeBuildPath(sourceDir, target.Context)
	if err != nil {
		return err
	}
	args := []string{"build", "-f", dockerfile}

	for _, tag := range tags {
		args = append(args, "-t", tag)
	}
	for key, value := range target.BuildArgs {
		args = append(args, "--build-arg", key+"="+value)
	}
	args = append(args, buildContext)

	return stream(ctx, sourceDir, log, dockerBinary, args...)
}

func safeBuildPath(sourceDir, requested string) (string, error) {
	resolved, err := filepath.Abs(filepath.Join(sourceDir, requested))
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(sourceDir)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the source directory", requested)
	}
	return resolved, nil
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
