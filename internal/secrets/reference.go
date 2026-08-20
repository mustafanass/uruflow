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

package secrets

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	Prefix = "${secret:"
	Suffix = "}"
	Masked = "••••••••"
)

var pattern = regexp.MustCompile(`\$\{secret:([A-Za-z0-9_.-]+)\}`)

type Lookup func(name string) (string, error)

func References(value string) []string {
	matches := pattern.FindAllStringSubmatch(value, -1)
	names := make([]string, 0, len(matches))

	for _, match := range matches {
		names = append(names, match[1])
	}
	return names
}

func HasReference(value string) bool {
	return pattern.MatchString(value)
}

func Names(env map[string]string) []string {
	unique := make(map[string]bool)
	for _, value := range env {
		for _, name := range References(value) {
			unique[name] = true
		}
	}

	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Resolve(env map[string]string, lookup Lookup) (map[string]string, error) {
	if len(env) == 0 {
		return env, nil
	}

	resolved := make(map[string]string, len(env))
	for key, value := range env {
		expanded, err := expand(value, lookup)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		resolved[key] = expanded
	}
	return resolved, nil
}

func Mask(value string) string {
	return pattern.ReplaceAllString(value, Masked)
}

func expand(value string, lookup Lookup) (string, error) {
	var failure error

	expanded := pattern.ReplaceAllStringFunc(value, func(match string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(match, Prefix), Suffix)

		secret, err := lookup(name)
		if err != nil {
			if failure == nil {
				failure = err
			}
			return match
		}
		return secret
	})

	if failure != nil {
		return "", failure
	}
	return expanded, nil
}
