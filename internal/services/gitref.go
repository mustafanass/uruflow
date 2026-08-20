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

package services

import "strings"

const gitSuffix = ".git"

func normalizeGitURL(url string) string {
	value := strings.TrimSpace(strings.ToLower(url))
	if value == "" {
		return ""
	}

	for _, scheme := range []string{"ssh://", "git://", "https://", "http://"} {
		value = strings.TrimPrefix(value, scheme)
	}

	if at := strings.LastIndex(value, "@"); at >= 0 {
		value = value[at+1:]
	}

	value = strings.Replace(value, ":", "/", 1)
	value = strings.TrimSuffix(value, gitSuffix)
	value = strings.Trim(value, "/")

	return value
}

func sameRepository(candidate string, identities []string) bool {
	target := normalizeGitURL(candidate)
	if target == "" {
		return false
	}

	for _, identity := range identities {
		if normalized := normalizeGitURL(identity); normalized != "" && normalized == target {
			return true
		}
	}
	return false
}
