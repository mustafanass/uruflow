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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

const (
	manifestMediaType = "application/vnd.docker.distribution.manifest.v2+json"
	catalogPageSize   = 200
	apiTimeout        = 15 * time.Second
)

func (r *Registry) Health(ctx context.Context) error {
	response, err := r.call(ctx, http.MethodGet, "/v2/")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("registry health: %s", response.Status)
	}
	return nil
}

func (r *Registry) Repositories(ctx context.Context) ([]string, error) {
	var catalog struct {
		Repositories []string `json:"repositories"`
	}
	if err := r.decode(ctx, fmt.Sprintf("/v2/_catalog?n=%d", catalogPageSize), &catalog); err != nil {
		return nil, err
	}

	sort.Strings(catalog.Repositories)
	return catalog.Repositories, nil
}

func (r *Registry) Tags(ctx context.Context, repository string) ([]string, error) {
	var listing struct {
		Tags []string `json:"tags"`
	}
	if err := r.decode(ctx, "/v2/"+repository+"/tags/list", &listing); err != nil {
		return nil, err
	}

	sort.Sort(sort.Reverse(sort.StringSlice(listing.Tags)))
	return listing.Tags, nil
}

func (r *Registry) Images(ctx context.Context) ([]models.Image, error) {
	repositories, err := r.Repositories(ctx)
	if err != nil {
		return nil, err
	}

	images := make([]models.Image, 0)
	for _, repository := range repositories {
		tags, err := r.Tags(ctx, repository)
		if err != nil {
			continue
		}
		for _, tag := range tags {
			image := models.Image{Repository: repository, Tag: tag}
			if digest, size, err := r.manifest(ctx, repository, tag); err == nil {
				image.Digest = digest
				image.Size = size
			}
			images = append(images, image)
		}
	}
	return images, nil
}

func (r *Registry) DeleteTag(ctx context.Context, repository, tag string) error {
	digest, _, err := r.manifest(ctx, repository, tag)
	if err != nil {
		return err
	}

	response, err := r.call(ctx, http.MethodDelete, "/v2/"+repository+"/manifests/"+digest)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)

	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("delete %s:%s: %s", repository, tag, response.Status)
	}
	return nil
}

func (r *Registry) manifest(ctx context.Context, repository, tag string) (string, int64, error) {
	response, err := r.call(ctx, http.MethodGet, "/v2/"+repository+"/manifests/"+tag)
	if err != nil {
		return "", 0, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, response.Body)
		return "", 0, fmt.Errorf("manifest %s:%s: %s", repository, tag, response.Status)
	}

	var manifest struct {
		Config struct {
			Size int64 `json:"size"`
		} `json:"config"`
		Layers []struct {
			Size int64 `json:"size"`
		} `json:"layers"`
	}
	if err := json.NewDecoder(response.Body).Decode(&manifest); err != nil {
		return "", 0, err
	}

	size := manifest.Config.Size
	for _, layer := range manifest.Layers {
		size += layer.Size
	}

	return response.Header.Get("Docker-Content-Digest"), size, nil
}

func (r *Registry) decode(ctx context.Context, path string, target any) error {
	response, err := r.call(ctx, http.MethodGet, path)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, response.Body)
		return fmt.Errorf("registry %s: %s", path, response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func (r *Registry) call(ctx context.Context, method, path string) (*http.Response, error) {
	client, err := r.httpClient()
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, method, "https://"+r.options.Address+path, nil)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(r.options.Username, r.options.Password)
	request.Header.Set("Accept", manifestMediaType)

	return client.Do(request)
}

func (r *Registry) httpClient() (*http.Client, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(r.options.CACert)) {
		return nil, fmt.Errorf("registry: cannot trust the uruflow CA")
	}

	return &http.Client{
		Timeout: apiTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		},
	}, nil
}

func (r *Registry) RepositoryName(project string) string {
	return fmt.Sprintf("%s/%s", r.options.Namespace, project)
}

func (r *Registry) ImageRepository(project string) string {
	return fmt.Sprintf("%s/%s", r.options.Address, r.RepositoryName(project))
}

func ShortDigest(digest string) string {
	_, hash, found := strings.Cut(digest, ":")
	if !found || len(hash) < 12 {
		return digest
	}
	return hash[:12]
}
