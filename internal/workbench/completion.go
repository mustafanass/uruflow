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
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/ops"
)

type choiceProjector func(prefix string, event ops.Event) []commandSpec

type argumentContext struct {
	Key       string
	Prefix    string
	Query     string
	Label     string
	Trail     string
	Request   []string
	Items     []commandSpec
	EmptyTip  string
	AllowRaw  bool
	NeedsNext bool
	Source    grammar.Source
}

type completionMsg struct {
	key   string
	items []commandSpec
	err   error
}

func commandWith(prefix string, arguments ...string) string {
	parts := []string{strings.TrimSpace(prefix)}
	parts = append(parts, arguments...)
	return strings.Join(parts, " ")
}

func argumentCompletion(value string) (argumentContext, bool) {
	raw := strings.TrimLeft(value, " \t")
	lower := strings.ToLower(raw)
	for _, command := range grammar.Visible() {
		if len(command.Arguments) == 0 {
			continue
		}
		path := grammar.Path(command)
		commandPrefix := path + " "
		if !strings.HasPrefix(lower, commandPrefix) {
			continue
		}
		return flowCompletion(command, raw[len(commandPrefix):])
	}
	return argumentContext{}, false
}

func flowCompletion(command grammar.Command, remainder string) (argumentContext, bool) {
	fields := strings.Fields(remainder)
	trailing := strings.HasSuffix(remainder, " ") || strings.HasSuffix(remainder, "\t")
	completed := make([]string, 0, len(fields))
	offset := 0
	for index, step := range command.Arguments {
		prefixMatched := hasFieldsAt(fields, offset, step.Prefix)
		if prefixMatched {
			offset += len(step.Prefix)
		}
		width := max(1, step.Width)
		available := len(fields) - offset
		last := index == len(command.Arguments)-1
		if last {
			_, err := grammar.Resolve(append(append([]string{}, command.Path...), fields...))
			return stepContext(command, index, step, completed, fields[offset:], err == nil), true
		}
		complete := available > width || available >= width && trailing || step.Advance && available >= width
		if len(step.Prefix) > 0 && !prefixMatched {
			complete = false
		}
		if !complete {
			return stepContext(command, index, step, completed, fields[offset:], false), true
		}
		completed = append(completed, fields[offset:offset+width]...)
		offset += width
	}
	return argumentContext{}, false
}

func stepContext(command grammar.Command, index int, step grammar.Argument, completed, query []string, allowRaw bool) argumentContext {
	prefixParts := append([]string{}, command.Path...)
	prefixParts = append(prefixParts, completed...)
	prefixParts = append(prefixParts, step.Prefix...)
	prefix := strings.Join(prefixParts, " ") + " "
	completion := argumentContext{
		Key:    grammar.Path(command) + ":" + strings.Join(completed, ":") + ":" + step.Label,
		Prefix: prefix, Query: strings.Join(query, " "), Label: step.Label,
		Trail:   completionTrail(command, index),
		Request: step.Request, EmptyTip: step.EmptyTip, AllowRaw: allowRaw,
		NeedsNext: index+1 < len(command.Arguments), Source: step.Source,
	}
	for _, item := range step.Choices {
		completion.Items = append(completion.Items, commandSpec{
			Command: commandWith(prefix, item.Arguments...), Display: item.Display, Summary: item.Summary,
		})
	}
	if index+1 < len(command.Arguments) {
		for itemIndex := range completion.Items {
			completion.Items[itemIndex].NeedsArgs = true
		}
	}
	return completion
}

func completionTrail(command grammar.Command, index int) string {
	words := strings.Fields(strings.ToUpper(grammar.Path(command)))
	root := words[len(words)-1]
	parts := []string{root}
	for stepIndex := 0; stepIndex <= index; stepIndex++ {
		label := command.Arguments[stepIndex].Label
		switch label {
		case "RUN MODE", "LOG MODE":
			label = "MODE"
		case "NEW PROJECT":
			label = "PROJECT"
		case "NEW AGENT":
			label = "AGENT"
		case "AGENT ROLE":
			label = "ROLE"
		case "YAML FILE":
			label = "YAML"
		case "PROJECT ENVIRONMENT":
			label = "PROJECT"
		}
		if parts[len(parts)-1] != label {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, " › ")
}

func hasFieldsAt(fields []string, offset int, wanted []string) bool {
	if len(wanted) == 0 || offset+len(wanted) > len(fields) {
		return false
	}
	for index := range wanted {
		if fields[offset+index] != wanted[index] {
			return false
		}
	}
	return true
}

func (m *model) requestArgumentCompletion() tea.Cmd {
	completion, ok := argumentCompletion(m.input.Value())
	if !ok || m.client == nil || len(completion.Items) > 0 || len(completion.Request) == 0 {
		return nil
	}
	if m.completionLoading == nil {
		m.completionLoading = make(map[string]bool)
	}
	if m.completionCache == nil {
		m.completionCache = make(map[string][]commandSpec)
	}
	if m.completionErrors == nil {
		m.completionErrors = make(map[string]string)
	}
	if m.completionLoading[completion.Key] {
		return nil
	}
	if _, cached := m.completionCache[completion.Key]; cached {
		return nil
	}
	if m.completionErrors[completion.Key] != "" {
		return nil
	}
	m.completionLoading[completion.Key] = true
	return func() tea.Msg {
		var items []commandSpec
		project := sourceProjector(completion.Source)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := m.client.Execute(ctx, completion.Request, "", func(event ops.Event) error {
			if event.Type != ops.EventTable || project == nil {
				return nil
			}
			items = append(items, project(completion.Prefix, event)...)
			return nil
		})
		if completion.NeedsNext {
			for index := range items {
				items[index].NeedsArgs = true
			}
		}
		return completionMsg{key: completion.Key, items: deduplicateChoices(items), err: err}
	}
}

func sourceProjector(source grammar.Source) choiceProjector {
	switch source {
	case grammar.SourceAgents:
		return tableChoices([]string{"NAME"}, "NAME", []string{"ROLES", "STATE", "VERSION"})
	case grammar.SourceProjects:
		return tableChoices([]string{"NAME"}, "NAME", []string{"WORKFLOW", "ENV", "SOURCE"})
	case grammar.SourceProjectEnvironments:
		return projectEnvironmentChoices
	case grammar.SourceReleases:
		return tableChoices([]string{"ID"}, "ID", []string{"PROJECT", "STATE", "AGE"})
	case grammar.SourceContainers:
		return containerChoices
	case grammar.SourceRegistry:
		return registryChoices
	case grammar.SourceAlerts:
		return tableChoices([]string{"ID"}, "ID", []string{"SEVERITY", "AGENT", "MESSAGE"})
	default:
		return nil
	}
}

func (m *model) retryArgumentCompletion() tea.Cmd {
	completion, ok := argumentCompletion(m.input.Value())
	if !ok {
		return nil
	}
	delete(m.completionErrors, completion.Key)
	delete(m.completionCache, completion.Key)
	delete(m.completionLoading, completion.Key)
	return m.requestArgumentCompletion()
}

func (m *model) suggestions() []commandSpec {
	if m.running || m.paste || m.confirm {
		return nil
	}
	if completion, ok := argumentCompletion(m.input.Value()); ok {
		if len(completion.Items) > 0 {
			return filterArgumentCommands(completion.Items, completion)
		}
		return filterArgumentCommands(m.completionCache[completion.Key], completion)
	}
	return matchingCommands(m.input.Value(), 0)
}

func (m *model) visibleSuggestions() ([]commandSpec, int) {
	all := m.suggestions()
	if len(all) == 0 {
		return nil, 0
	}
	limit := 6
	if m.height > 0 && m.height < 18 {
		limit = 3
	}
	if len(all) <= limit {
		return all, 0
	}
	selected := min(m.suggestionAt, len(all)-1)
	start := max(0, selected-limit+1)
	if start+limit > len(all) {
		start = len(all) - limit
	}
	return all[start : start+limit], start
}

func (m *model) completeSuggestion() bool {
	all := m.suggestions()
	if len(all) == 0 {
		return false
	}
	m.setCompletion(all[min(m.suggestionAt, len(all)-1)])
	return true
}

func (m *model) setCompletion(item commandSpec) {
	value := item.Command
	if item.NeedsArgs {
		value += " "
	}
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.suggestionAt = 0
	m.resize()
}
