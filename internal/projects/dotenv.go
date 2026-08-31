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
	"bufio"
	"fmt"
	"strings"
)

const exportPrefix = "export "

func ParseDotEnv(content string) (map[string]string, error) {
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))

	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		text = strings.TrimPrefix(text, exportPrefix)

		key, value, found := strings.Cut(text, "=")
		if !found {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE, got %q", line, text)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("line %d: empty variable name", line)
		}
		if !validKey(key) {
			return nil, fmt.Errorf("line %d: %q is not a valid variable name", line, key)
		}

		values[key] = unquote(strings.TrimSpace(value))
	}

	return values, scanner.Err()
}

func validKey(key string) bool {
	for index, symbol := range key {
		switch {
		case symbol >= 'A' && symbol <= 'Z', symbol >= 'a' && symbol <= 'z', symbol == '_':
		case symbol >= '0' && symbol <= '9' && index > 0:
		default:
			return false
		}
	}
	return key != ""
}

func unquote(value string) string {
	if len(value) < 2 {
		return value
	}

	first, last := value[0], value[len(value)-1]
	if first != last {
		return value
	}

	switch first {
	case '"':
		return strings.NewReplacer(`\n`, "\n", `\"`, `"`, `\\`, `\`).Replace(value[1 : len(value)-1])
	case '\'':
		return value[1 : len(value)-1]
	default:
		return value
	}
}
