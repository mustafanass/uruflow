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
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	manifestMediaType    = "application/vnd.docker.distribution.manifest.v2+json"
	ociManifestMediaType = "application/vnd.oci.image.manifest.v1+json"
	ociIndexMediaType    = "application/vnd.oci.image.index.v1+json"
	dockerListMediaType  = "application/vnd.docker.distribution.manifest.list.v2+json"
	catalogPageSize      = 200
	apiTimeout           = 15 * time.Second
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
	path := fmt.Sprintf("/v2/_catalog?n=%d", catalogPageSize)
	seen := make(map[string]struct{})
	unique := make(map[string]struct{})
	var repositories []string
	for path != "" {
		if _, exists := seen[path]; exists {
			return nil, fmt.Errorf("registry catalog pagination repeated %q", path)
		}
		seen[path] = struct{}{}
		page, next, err := r.catalog(ctx, path)
		if err != nil {
			return nil, err
		}
		for _, repository := range page {
			if _, exists := unique[repository]; exists {
				continue
			}
			unique[repository] = struct{}{}
			repositories = append(repositories, repository)
		}
		path = next
	}

	sort.Strings(repositories)
	return repositories, nil
}

func (r *Registry) catalog(ctx context.Context, path string) ([]string, string, error) {
	var catalog struct {
		Repositories []string `json:"repositories"`
	}
	response, err := r.call(ctx, http.MethodGet, path)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, response.Body)
		return nil, "", fmt.Errorf("registry %s: %s", path, response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(&catalog); err != nil {
		return nil, "", err
	}
	next, err := catalogNext(response.Header.Get("Link"))
	if err != nil {
		return nil, "", err
	}
	return catalog.Repositories, next, nil
}

func catalogNext(header string) (string, error) { return pageNext(header, "/v2/_catalog") }

func pageNext(header, path string) (string, error) {
	for _, value := range strings.Split(header, ",") {
		parts := strings.Split(value, ";")
		if len(parts) < 2 {
			continue
		}
		next := false
		for _, parameter := range parts[1:] {
			if strings.EqualFold(strings.TrimSpace(parameter), `rel="next"`) {
				next = true
				break
			}
		}
		if !next {
			continue
		}
		target := strings.TrimSpace(parts[0])
		if len(target) < 3 || target[0] != '<' || target[len(target)-1] != '>' {
			return "", fmt.Errorf("registry catalog returned an invalid next link")
		}
		parsed, err := url.ParseRequestURI(target[1 : len(target)-1])
		if err != nil || parsed.IsAbs() || parsed.Path != path || parsed.Host != "" {
			return "", fmt.Errorf("registry catalog returned an invalid next link")
		}
		return parsed.RequestURI(), nil
	}
	return "", nil
}

func (r *Registry) Tags(ctx context.Context, repository string) ([]string, error) {
	if !validRepository(repository) {
		return nil, fmt.Errorf("invalid repository")
	}
	base := "/v2/" + repository + "/tags/list"
	path := fmt.Sprintf("%s?n=%d", base, catalogPageSize)
	seen := map[string]bool{}
	tags := map[string]bool{}
	for path != "" {
		if seen[path] {
			return nil, fmt.Errorf("registry tag pagination repeated")
		}
		seen[path] = true
		response, err := r.call(ctx, http.MethodGet, path)
		if err != nil {
			return nil, err
		}
		var listing struct {
			Tags []string `json:"tags"`
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("registry tags: %s", response.Status)
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&listing)
		next, nextErr := pageNext(response.Header.Get("Link"), base)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		if nextErr != nil {
			return nil, nextErr
		}
		for _, tag := range listing.Tags {
			if validTag(tag) {
				tags[tag] = true
			}
		}
		path = next
	}
	result := make([]string, 0, len(tags))
	for tag := range tags {
		result = append(result, tag)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(result)))
	return result, nil
}

func (r *Registry) DeleteTag(ctx context.Context, repository, tag string) error {
	manifest, err := r.readManifest(ctx, repository, tag)
	if err != nil {
		return err
	}

	response, err := r.call(ctx, http.MethodDelete, "/v2/"+repository+"/manifests/"+manifest.Digest)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)

	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("delete %s:%s: %s", repository, tag, response.Status)
	}
	r.RefreshCatalog()
	return nil
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
	username, password := r.credentials()
	request.SetBasicAuth(username, password)
	request.Header.Set("Accept", strings.Join([]string{ociIndexMediaType, ociManifestMediaType, dockerListMediaType, manifestMediaType}, ", "))

	return client.Do(request)
}

func (r *Registry) httpClient() (*http.Client, error) {
	r.clientOnce.Do(func() {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(r.options.CACert)) {
			r.clientErr = fmt.Errorf("registry: cannot trust the uruflow CA")
			return
		}
		r.client = &http.Client{
			Timeout: apiTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
			},
		}
	})
	return r.client, r.clientErr
}

func (r *Registry) RepositoryName(project string) string {
	return fmt.Sprintf("%s/%s", r.options.Namespace, project)
}

func (r *Registry) ImageRepository(project string) string {
	return fmt.Sprintf("%s/%s", r.options.Address, r.RepositoryName(project))
}
