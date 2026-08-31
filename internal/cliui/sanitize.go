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

package cliui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/mustafanass/uruflow/internal/ops"
)

func SafeText(value string) string {
	value = ansi.Strip(value)
	return strings.Map(func(char rune) rune {
		switch char {
		case '\n', '\r', '\t':
			return ' '
		}
		if unicode.IsControl(char) || unicode.In(char, unicode.Cf) {
			return -1
		}
		return char
	}, value)
}

func safeEvent(event ops.Event) ops.Event {
	event.Type = SafeText(event.Type)
	event.Level = SafeText(event.Level)
	event.Operation = SafeText(event.Operation)
	event.Title = SafeText(event.Title)
	event.Message = SafeText(event.Message)
	columns := make([]string, len(event.Columns))
	for index := range event.Columns {
		columns[index] = SafeText(event.Columns[index])
	}
	event.Columns = columns
	rows := make([][]string, len(event.Rows))
	for row := range event.Rows {
		rows[row] = make([]string, len(event.Rows[row]))
		for column := range event.Rows[row] {
			rows[row][column] = SafeText(event.Rows[row][column])
		}
	}
	event.Rows = rows
	event.Data = safeValue(event.Data)
	return event
}

func safeValue(value any) any {
	switch typed := value.(type) {
	case string:
		return SafeText(typed)
	case []string:
		result := make([]string, len(typed))
		for index := range typed {
			result[index] = SafeText(typed[index])
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = safeValue(typed[index])
		}
		return result
	case map[string]string:
		result := make(map[string]string, len(typed))
		for key, item := range typed {
			result[SafeText(key)] = SafeText(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[SafeText(key)] = safeValue(item)
		}
		return result
	default:
		return value
	}
}
