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

	"gopkg.in/yaml.v3"
)

type CreationDocument struct {
	Project     Definition  `yaml:"project"`
	Environment Environment `yaml:"environment"`
}

func ParseCreationYAML(content string) (CreationDocument, error) {
	var document CreationDocument
	if strings.TrimSpace(content) == "" {
		return document, errors.New("project YAML cannot be empty")
	}
	if err := decodeStrict([]byte(content), &document); err != nil {
		return document, err
	}
	environment, err := yaml.Marshal(document.Environment)
	if err != nil {
		return document, err
	}
	if err := ValidateEnvironmentYAML(string(environment)); err != nil {
		return document, err
	}
	return document, nil
}
