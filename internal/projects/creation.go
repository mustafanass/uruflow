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

package projects

import (
	"errors"
	"strings"
)

func ParseCreationYAML(content string) (Environment, error) {
	var environment Environment
	if strings.TrimSpace(content) == "" {
		return environment, errors.New("environment YAML cannot be empty")
	}
	if err := decodeStrict([]byte(content), &environment); err != nil {
		return environment, err
	}
	if err := ValidateEnvironmentYAML(content); err != nil {
		return environment, err
	}
	return environment, nil
}
