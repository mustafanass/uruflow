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

package roles

import (
	"fmt"
	"strings"

	"github.com/mustafanass/uruflow/internal/ufp"
)

func Parse(value string) ([]ufp.Role, error) {
	parsed := make([]ufp.Role, 0, 2)

	for _, part := range strings.Split(value, ",") {
		role := ufp.Role(strings.ToLower(strings.TrimSpace(part)))
		if role == "" {
			continue
		}
		if !role.Valid() {
			return nil, fmt.Errorf("unknown role %q: use builder, runner or both", part)
		}
		if !ufp.HasRole(parsed, role) {
			parsed = append(parsed, role)
		}
	}

	if len(parsed) == 0 {
		return nil, fmt.Errorf("at least one role is required: builder, runner")
	}
	return parsed, nil
}

func Format(value []ufp.Role) string {
	names := make([]string, 0, len(value))
	for _, role := range value {
		names = append(names, string(role))
	}
	return strings.Join(names, ", ")
}
