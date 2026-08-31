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

package workbench

import (
	"strings"

	"github.com/mustafanass/uruflow/internal/grammar"
)

type commandSpec struct {
	Command   string
	Display   string
	Summary   string
	NeedsArgs bool
}

var commandCatalog = buildCommandCatalog()

func buildCommandCatalog() []commandSpec {
	commands := grammar.Visible()
	result := make([]commandSpec, 0, len(commands))
	for _, command := range commands {
		result = append(result, commandSpec{
			Command: grammar.Path(command), Summary: command.Summary, NeedsArgs: len(command.Arguments) > 0,
		})
	}
	return result
}

func matchingCommands(value string, limit int) []commandSpec {
	hadTrailingSpace := strings.HasSuffix(value, " ") || strings.HasSuffix(value, "\t")
	raw := strings.TrimSpace(value)
	palette := strings.HasPrefix(raw, "/")
	query := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "/")))
	if query == "" && !palette {
		return nil
	}
	if hadTrailingSpace {
		for _, item := range commandCatalog {
			if strings.EqualFold(query, item.Command) && item.NeedsArgs {
				return nil
			}
		}
	}

	for _, item := range commandCatalog {
		if item.NeedsArgs && strings.HasPrefix(query, item.Command+" ") {
			return nil
		}
	}

	var prefix, word, contains []commandSpec
	for _, item := range commandCatalog {
		command := strings.ToLower(item.Command)
		summary := strings.ToLower(item.Summary)
		switch {
		case query == "" || strings.HasPrefix(command, query):
			prefix = append(prefix, item)
		case wordHasPrefix(command, query):
			word = append(word, item)
		case strings.Contains(command, query) || strings.Contains(summary, query):
			contains = append(contains, item)
		}
	}
	result := append(prefix, word...)
	result = append(result, contains...)
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func wordHasPrefix(value, query string) bool {
	for _, field := range strings.Fields(value) {
		if strings.HasPrefix(field, query) {
			return true
		}
	}
	return false
}
