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

	"github.com/mustafanass/uruflow/internal/cliui"
	"github.com/mustafanass/uruflow/internal/ops"
)

func tableChoices(arguments []string, display string, details []string) choiceProjector {
	return func(prefix string, event ops.Event) []commandSpec {
		items := make([]commandSpec, 0, len(event.Rows))
		for _, row := range event.Rows {
			values := tableRow(event.Columns, row)
			args := valuesFor(values, arguments)
			if len(args) != len(arguments) || containsBlank(args) {
				continue
			}
			items = append(items, commandSpec{
				Command: commandWith(prefix, args...), Display: values[display], Summary: joinValues(values, details),
			})
		}
		return items
	}
}

func projectEnvironmentChoices(prefix string, event ops.Event) []commandSpec {
	items := make([]commandSpec, 0, len(event.Rows))
	for _, row := range event.Rows {
		values := tableRow(event.Columns, row)
		name, environment := values["NAME"], values["ENV"]
		if name == "" || environment == "" {
			continue
		}
		project := strings.TrimSuffix(name, "-"+environment)
		items = append(items, commandSpec{
			Command: commandWith(prefix, project, environment), Display: project + " · " + environment,
			Summary: joinValues(values, []string{"WORKFLOW", "BRANCH", "SOURCE"}),
		})
	}
	return items
}

func containerChoices(prefix string, event ops.Event) []commandSpec {
	items := make([]commandSpec, 0, len(event.Rows))
	for _, row := range event.Rows {
		values := tableRow(event.Columns, row)
		agent, id := values["AGENT"], values["ID"]
		if agent == "" || id == "" {
			continue
		}
		name := values["NAME"]
		if name == "" {
			name = id
		}
		items = append(items, commandSpec{
			Command: commandWith(prefix, agent, id), Display: name + " · " + id,
			Summary: joinValues(values, []string{"AGENT", "TYPE", "PROJECT", "SERVICE", "STATE"}),
		})
	}
	return items
}

func registryChoices(prefix string, event ops.Event) []commandSpec {
	items := make([]commandSpec, 0, len(event.Rows))
	for _, row := range event.Rows {
		values := tableRow(event.Columns, row)
		repository, tag := values["REPOSITORY"], values["TAG"]
		if repository == "" || tag == "" {
			continue
		}
		items = append(items, commandSpec{
			Command: commandWith(prefix, repository, tag), Display: repository + ":" + tag,
			Summary: joinValues(values, []string{"SIZE", "AGE", "DIGEST"}),
		})
	}
	return items
}

func tableRow(columns, row []string) map[string]string {
	values := make(map[string]string, len(columns))
	for index, column := range columns {
		if index < len(row) {
			values[column] = cliui.SafeText(row[index])
		}
	}
	return values
}

func valuesFor(values map[string]string, columns []string) []string {
	result := make([]string, 0, len(columns))
	for _, column := range columns {
		result = append(result, values[column])
	}
	return result
}

func containsBlank(values []string) bool {
	for _, value := range values {
		if value == "" || value == "–" {
			return true
		}
	}
	return false
}

func joinValues(values map[string]string, columns []string) string {
	result := make([]string, 0, len(columns))
	for _, column := range columns {
		value := values[column]
		if value != "" && value != "–" {
			result = append(result, value)
		}
		if len(result) == 3 {
			break
		}
	}
	return strings.Join(result, " · ")
}

func deduplicateChoices(items []commandSpec) []commandSpec {
	seen := make(map[string]bool, len(items))
	result := make([]commandSpec, 0, len(items))
	for _, item := range items {
		if seen[item.Command] {
			continue
		}
		seen[item.Command] = true
		result = append(result, item)
	}
	return result
}

func filterArgumentCommands(items []commandSpec, completion argumentContext) []commandSpec {
	query := strings.Fields(strings.ToLower(completion.Query))
	if len(query) == 0 {
		return items
	}
	filtered := make([]commandSpec, 0, len(items))
	for _, item := range items {
		value := strings.TrimPrefix(item.Command, completion.Prefix)
		haystack := strings.ToLower(strings.Join([]string{value, item.Display, item.Summary}, " "))
		matches := true
		for _, word := range query {
			if !strings.Contains(haystack, word) {
				matches = false
				break
			}
		}
		if matches {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
