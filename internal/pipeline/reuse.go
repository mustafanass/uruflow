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

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func (p *Pipeline) reuseBuilds(project *models.Project, targets []ufp.BuildTarget) error {
	previous, err := p.store.LastSuccessfulRelease(project.Name)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if previous.Spec.Builder != project.Builder {
		return nil
	}
	old := make(map[string]ufp.BuildTarget)
	for _, target := range p.buildTargets(&previous.Spec) {
		old[target.Service] = target
	}
	for i := range targets {
		target := &targets[i]
		prior, ok := old[target.Service]
		if !ok || !sameBuild(*target, prior) {
			continue
		}
		image, commit := previous.Images[target.Service], previous.Commits[target.Service]
		if !models.ValidDigestReference(image) || !strings.HasPrefix(image, target.Image+"@sha256:") || !models.ValidGitCommit(commit) {
			continue
		}
		target.PreviousImage, target.PreviousCommit = image, commit
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return p.checkReusableImages(ctx, targets)
}

func (p *Pipeline) checkReusableImages(ctx context.Context, targets []ufp.BuildTarget) error {
	jobs := make(chan int, len(targets))
	for index, target := range targets {
		if target.PreviousImage != "" {
			jobs <- index
		}
	}
	close(jobs)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var group sync.WaitGroup
	var once sync.Once
	var failure error
	workers := min(8, len(jobs))
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				target := &targets[index]
				exists, err := p.registry.HasImage(ctx, target.PreviousImage)
				if err != nil {
					once.Do(func() {
						failure = fmt.Errorf("%w: check reusable image for service %q: %v", ErrRegistryUnavailable, target.Service, err)
						cancel()
					})
					return
				}
				if !exists {
					target.PreviousImage, target.PreviousCommit = "", ""
				}
			}
		}()
	}
	group.Wait()
	if failure != nil {
		return failure
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: image checks: %v", ErrRegistryUnavailable, err)
	}
	return nil
}

func sameBuild(a, b ufp.BuildTarget) bool {
	return a.Image == b.Image && a.GitURL == b.GitURL && a.Branch == b.Branch &&
		a.Dockerfile == b.Dockerfile && a.Context == b.Context &&
		maps.Equal(a.BuildArgs, b.BuildArgs) && slices.Equal(a.Watch, b.Watch)
}
