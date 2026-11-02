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

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const catalogInterval = 3 * time.Minute
const catalogTimeout = 10 * time.Second
const catalogWorkers = 8

type catalogCache struct {
	mu    sync.RWMutex
	once  sync.Once
	wake  chan struct{}
	value models.ImageCatalog
}

func (r *Registry) StartCatalog(ctx context.Context) {
	r.catalogCache.once.Do(func() {
		r.catalogCache.mu.Lock()
		r.catalogCache.value.Refreshing = true
		r.catalogCache.mu.Unlock()
		go func() {
			timer := time.NewTicker(catalogInterval)
			defer timer.Stop()
			for {
				r.refreshCatalog(ctx)
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				case <-r.catalogCache.wake:
				}
			}
		}()
	})
}
func (r *Registry) RefreshCatalog() {
	r.catalogCache.mu.Lock()
	r.catalogCache.value.Refreshing = true
	r.catalogCache.mu.Unlock()
	select {
	case r.catalogCache.wake <- struct{}{}:
	default:
	}
}
func (r *Registry) Images(context.Context) (models.ImageCatalog, error) {
	r.catalogCache.mu.RLock()
	value := r.catalogCache.value
	value.Images = append([]models.Image{}, value.Images...)
	r.catalogCache.mu.RUnlock()
	if value.SampledAt.IsZero() && !value.Refreshing && value.Error == "" {
		value.Error = "Registry catalog is not ready."
	}
	return value, nil
}
func (r *Registry) refreshCatalog(parent context.Context) {
	if parent.Err() != nil {
		return
	}
	r.catalogCache.mu.Lock()
	r.catalogCache.value.Refreshing = true
	r.catalogCache.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, catalogTimeout)
	defer cancel()
	images, partial, err := r.listImages(ctx)
	r.catalogCache.mu.Lock()
	defer r.catalogCache.mu.Unlock()
	r.catalogCache.value.Refreshing = false
	if err != nil {
		r.catalogCache.value.Error = "Registry refresh failed. Showing the last catalog."
		logger.Warn("[REGISTRY] catalog refresh: %v", err)
		return
	}
	r.catalogCache.value = models.ImageCatalog{Images: images, SampledAt: time.Now(), Partial: partial}
	r.saveCatalogLocked()
}
func (r *Registry) listImages(ctx context.Context) ([]models.Image, bool, error) {
	repositories, err := r.Repositories(ctx)
	if err != nil {
		return nil, false, err
	}
	groups := make([][]models.Image, len(repositories))
	failed := make([]bool, len(repositories))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for i := 0; i < catalogWorkers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				tags, err := r.Tags(ctx, repositories[index])
				if err != nil {
					failed[index] = true
					continue
				}
				for _, tag := range tags {
					groups[index] = append(groups[index], models.Image{Repository: repositories[index], Tag: tag})
				}
			}
		}()
	}
	for index := range repositories {
		select {
		case jobs <- index:
		case <-ctx.Done():
			failed[index] = true
		}
	}
	close(jobs)
	workers.Wait()
	images := []models.Image{}
	partial := false
	for index, group := range groups {
		images = append(images, group...)
		partial = partial || failed[index]
	}
	if partial && len(images) == 0 && len(repositories) > 0 {
		return nil, true, fmt.Errorf("registry tag listing incomplete")
	}
	return images, partial, nil
}
func (r *Registry) catalogPath() string {
	if r.options.DataDir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(r.options.DataDir), "registry-catalog.json")
}
func (r *Registry) loadCatalog() {
	path := r.catalogPath()
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > 16<<20 {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var value models.ImageCatalog
	if json.Unmarshal(data, &value) == nil {
		value.Refreshing = false
		value.Error = ""
		r.catalogCache.value = value
	}
}
func (r *Registry) saveCatalogLocked() {
	path := r.catalogPath()
	if path == "" {
		return
	}
	data, err := json.Marshal(r.catalogCache.value)
	if err != nil {
		return
	}
	temporary := path + ".tmp"
	if err = os.WriteFile(temporary, data, 0600); err == nil {
		err = os.Rename(temporary, path)
	}
	if err != nil {
		logger.Warn("[REGISTRY] save catalog: %v", err)
	}
}
