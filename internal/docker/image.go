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

package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Auth struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	ServerAddress string `json:"serveraddress"`
}

type pullProgress struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (c *Client) Pull(ctx context.Context, image string, auth *Auth, onProgress func(string)) error {
	repository, tag := SplitTag(image)

	query := url.Values{}
	query.Set("fromImage", repository)
	query.Set("tag", tag)

	headers := map[string]string{}
	if auth != nil {
		encoded, err := json.Marshal(auth)
		if err != nil {
			return err
		}
		headers["X-Registry-Auth"] = base64.URLEncoding.EncodeToString(encoded)
	}

	response, err := c.do(ctx, http.MethodPost, "/images/create?"+query.Encode(), nil, headers)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	decoder := json.NewDecoder(response.Body)
	for {
		var progress pullProgress
		if err := decoder.Decode(&progress); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("pull %s: decode progress: %w", image, err)
		}
		if progress.Error != "" {
			return fmt.Errorf("pull %s: %s", image, progress.Error)
		}
		if onProgress != nil && progress.Status != "" {
			onProgress(progress.Status)
		}
	}

	return nil
}

func (c *Client) HasImage(ctx context.Context, image string) bool {
	return c.get(ctx, "/images/"+url.PathEscape(image)+"/json", nil) == nil
}

func SplitTag(image string) (repository, tag string) {
	if separator := strings.LastIndex(image, "@"); separator >= 0 {
		return image[:separator], image[separator+1:]
	}
	separator := strings.LastIndex(image, ":")
	if separator < 0 || strings.Contains(image[separator+1:], "/") {
		return image, "latest"
	}
	return image[:separator], image[separator+1:]
}
