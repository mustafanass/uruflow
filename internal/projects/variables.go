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

	"github.com/mustafanass/uruflow/internal/secrets"
)

const (
	variableEditorPlain  = "# Plain variable: NAME=value"
	variableEditorSecret = "# Secret variable: secret NAME=value"
	variableEditorNote   = "# Stored secret values reopen as references and are never revealed."
	secretLinePrefix     = "secret "
)

type VariableEdit struct {
	DotEnv  string
	Secrets map[string]string
}

func FormatVariableEditor(raw string) (string, error) {
	if _, err := ParseDotEnv(raw); err != nil {
		return "", err
	}
	lines := []string{variableEditorPlain, variableEditorSecret, variableEditorNote, ""}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		original := scanner.Text()
		text := strings.TrimSpace(original)
		if text == "" || strings.HasPrefix(text, "#") {
			lines = append(lines, original)
			continue
		}
		key, value, err := parseVariableAssignment(text)
		if err != nil {
			return "", err
		}
		if _, secret := secrets.ReferenceName(value); secret {
			lines = append(lines, secretLinePrefix+key+"="+value)
		} else {
			lines = append(lines, original)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n", nil
}

func ParseVariableEditor(input, namespace string) (VariableEdit, error) {
	if err := secrets.ValidateName(namespace); err != nil {
		return VariableEdit{}, err
	}
	edit := VariableEdit{Secrets: make(map[string]string)}
	seen := make(map[string]int)
	output := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(input))
	for line := 1; scanner.Scan(); line++ {
		original := scanner.Text()
		text := strings.TrimSpace(original)
		if text == "" {
			output = append(output, "")
			continue
		}
		if strings.HasPrefix(text, "#") {
			if text != variableEditorPlain && text != variableEditorSecret && text != variableEditorNote {
				output = append(output, original)
			}
			continue
		}
		secretLine := strings.HasPrefix(text, secretLinePrefix)
		if secretLine {
			text = strings.TrimSpace(strings.TrimPrefix(text, secretLinePrefix))
		}
		key, value, err := parseVariableAssignment(text)
		if err != nil {
			return VariableEdit{}, fmt.Errorf("line %d: %w", line, err)
		}
		if previous, duplicate := seen[key]; duplicate {
			return VariableEdit{}, fmt.Errorf("line %d: variable %q was already defined on line %d", line, key, previous)
		}
		seen[key] = line
		if !secretLine {
			output = append(output, original)
			continue
		}
		if name, existing := secrets.ReferenceName(value); existing {
			output = append(output, key+"="+secrets.Prefix+name+secrets.Suffix)
			continue
		}
		if value == "" {
			return VariableEdit{}, fmt.Errorf("line %d: secret variable %q has an empty value", line, key)
		}
		name := namespace + "." + key
		if err := secrets.ValidateName(name); err != nil {
			return VariableEdit{}, fmt.Errorf("line %d: %w", line, err)
		}
		edit.Secrets[name] = value
		output = append(output, key+"="+secrets.Prefix+name+secrets.Suffix)
	}
	if err := scanner.Err(); err != nil {
		return VariableEdit{}, err
	}
	edit.DotEnv = strings.TrimSpace(strings.Join(output, "\n"))
	if edit.DotEnv != "" {
		edit.DotEnv += "\n"
	}
	if _, err := ParseDotEnv(edit.DotEnv); err != nil {
		return VariableEdit{}, err
	}
	return edit, nil
}

func VariableSecretNames(raw string) ([]string, error) {
	values, err := ParseDotEnv(raw)
	if err != nil {
		return nil, err
	}
	return secrets.Names(values), nil
}

func parseVariableAssignment(text string) (string, string, error) {
	text = strings.TrimPrefix(text, exportPrefix)
	key, rawValue, found := strings.Cut(text, "=")
	if !found {
		return "", "", fmt.Errorf("expected NAME=value")
	}
	key = strings.TrimSpace(key)
	if !validKey(key) {
		return "", "", fmt.Errorf("%q is not a valid variable name", key)
	}
	return key, unquote(strings.TrimSpace(rawValue)), nil
}
