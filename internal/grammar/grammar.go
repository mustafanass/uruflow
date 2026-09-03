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

package grammar

import (
	"fmt"
	"strconv"
	"strings"
)

type Source string

const (
	SourceAgents              Source = "agents"
	SourceProjects            Source = "projects"
	SourceProjectEnvironments Source = "project_environments"
	SourceReleases            Source = "releases"
	SourceContainers          Source = "containers"
	SourceRegistry            Source = "registry"
	SourceAlerts              Source = "alerts"
)

type InputMode string

const (
	InputYAML      InputMode = "yaml"
	InputVariables InputMode = "variables"
)

type Action string

const (
	ActionClear Action = "clear"
	ActionExit  Action = "exit"
)

type Validation string

const (
	ValidationUint         Validation = "uint"
	ValidationLimit        Validation = "limit"
	ValidationContainerLog Validation = "container_log"
)

type Choice struct {
	Arguments []string
	Display   string
	Summary   string
}

type Argument struct {
	Name       string
	Label      string
	EmptyTip   string
	Usage      string
	Prefix     []string
	Width      int
	Optional   bool
	Advance    bool
	Source     Source
	Request    []string
	Choices    []Choice
	Validation Validation
}

type Command struct {
	Path           []string
	Summary        string
	Arguments      []Argument
	Input          InputMode
	Action         Action
	Confirm        bool
	Durable        bool
	Focused        bool
	ExternalEditor bool
	Hidden         bool
}

var commands = []Command{
	{Path: []string{"status"}, Summary: "Fleet health, agents and projects"},
	{Path: []string{"events"}, Summary: "Follow or resume sequenced fleet activity", Focused: true, Arguments: []Argument{
		{Name: "SEQUENCE", Label: "SEQUENCE", Usage: "[--after SEQUENCE]", Prefix: []string{"--after"}, Optional: true, Validation: ValidationUint},
	}},
	{Path: []string{"agent", "list"}, Summary: "List enrolled agents"},
	{Path: []string{"agent", "show"}, Summary: "Inspect an agent", Arguments: []Argument{resource("AGENT", SourceAgents, []string{"agent", "list"}, "No agents are enrolled.")}},
	{Path: []string{"agent", "add"}, Summary: "Enrol and show one-time credentials", Arguments: []Argument{
		{Name: "NAME", Label: "NEW AGENT", EmptyTip: "Type a lowercase agent name to continue.", Advance: true},
		{Name: "ROLES", Label: "AGENT ROLE", Usage: "[--roles builder|runner|builder,runner]", Prefix: []string{"--roles"}, Optional: true, Choices: []Choice{
			{Arguments: []string{"runner"}, Display: "runner", Summary: "Run deployed containers"},
			{Arguments: []string{"builder"}, Display: "builder", Summary: "Build and publish images"},
			{Arguments: []string{"builder,runner"}, Display: "builder,runner", Summary: "Build and run on this machine"},
		}},
	}},
	{Path: []string{"agent", "remove"}, Summary: "Remove an enrolled agent", Confirm: true, Arguments: []Argument{resource("AGENT", SourceAgents, []string{"agent", "list"}, "No agents are enrolled.")}},
	{Path: []string{"project", "list"}, Summary: "List YAML-owned projects"},
	{Path: []string{"project", "create"}, Summary: "Create a project environment", Input: InputYAML, Arguments: []Argument{
		{Name: "PROJECT", Label: "NEW PROJECT", EmptyTip: "Type a lowercase project name to continue.", Advance: true},
		{Name: "ENV", Label: "ENVIRONMENT", EmptyTip: "Type an environment name to open the project editor."},
	}},
	{Path: []string{"project", "show"}, Summary: "Inspect a project and its services", Arguments: []Argument{project()}},
	{Path: []string{"project", "edit"}, Summary: "Edit authoritative environment YAML", ExternalEditor: true, Arguments: []Argument{project()}},
	{Path: []string{"project", "path"}, Summary: "Resolve authoritative environment YAML", Hidden: true, Arguments: []Argument{project()}},
	{Path: []string{"project", "validate"}, Summary: "Validate environment YAML", Input: InputYAML, Arguments: []Argument{{Name: "FILE", Label: "YAML FILE", EmptyTip: "Type a YAML path, or - to open the inline editor."}}},
	{Path: []string{"project", "apply"}, Summary: "Validate and atomically apply environment YAML", Input: InputYAML, Arguments: []Argument{
		{Name: "PROJECT ENV", Label: "PROJECT ENVIRONMENT", Width: 2, Source: SourceProjectEnvironments, Request: []string{"project", "list"}, EmptyTip: "No loaded project environments. Add YAML, then run project reload."},
		{Name: "FILE", Label: "YAML FILE", EmptyTip: "Type a YAML path, or - to open the inline editor."},
	}},
	{Path: []string{"project", "reload"}, Summary: "Reload authoritative environment YAML"},
	{Path: []string{"project", "deploy"}, Summary: "Start and follow a project release", Durable: true, Arguments: []Argument{project(), runMode()}},
	{Path: []string{"project", "rollback"}, Summary: "Restore a project's previous release", Durable: true, Arguments: []Argument{project(), runMode()}},
	{Path: []string{"project", "stop"}, Summary: "Stop a project on every runner", Confirm: true, Arguments: []Argument{project()}},
	{Path: []string{"project", "variables"}, Summary: "Manage optional plain and secret variables", Input: InputVariables, Arguments: []Argument{project()}},
	{Path: []string{"project", "variables-source"}, Summary: "Load variables for the project editor", Hidden: true, Arguments: []Argument{project()}},
	{Path: []string{"release", "list"}, Summary: "List recent releases", Arguments: []Argument{{Name: "LIMIT", Label: "LIMIT", Usage: "[--limit N]", Prefix: []string{"--limit"}, Optional: true, Validation: ValidationLimit}}},
	{Path: []string{"release", "show"}, Summary: "Inspect a release", Arguments: []Argument{release()}},
	{Path: []string{"release", "logs"}, Summary: "Read or follow release output", Arguments: []Argument{release(), logMode()}},
	{Path: []string{"release", "follow"}, Summary: "Attach to a release", Arguments: []Argument{release()}},
	{Path: []string{"container", "list"}, Summary: "List managed containers"},
	{Path: []string{"container", "logs"}, Summary: "Stream application output", Arguments: []Argument{
		{Name: "AGENT CONTAINER", Label: "CONTAINER", Width: 2, Source: SourceContainers, Request: []string{"container", "list"}, EmptyTip: "No managed containers are reporting yet."},
		{Name: "MODE", Label: "LOG MODE", Usage: "[--tail N] [--follow]", Optional: true, Validation: ValidationContainerLog, Choices: []Choice{
			{Display: "recent · 200 lines", Summary: "Show recent output and return"},
			{Arguments: []string{"--follow"}, Display: "follow live", Summary: "Keep streaming new container output"},
			{Arguments: []string{"--tail", "100"}, Display: "recent · 100 lines", Summary: "Show the latest 100 lines"},
			{Arguments: []string{"--tail", "500"}, Display: "recent · 500 lines", Summary: "Show the latest 500 lines"},
		}},
	}},
	{Path: []string{"registry", "list"}, Summary: "List stored image manifests"},
	{Path: []string{"registry", "remove"}, Summary: "Delete an image manifest", Confirm: true, Arguments: []Argument{
		{Name: "REPOSITORY TAG", Label: "IMAGE", Width: 2, Source: SourceRegistry, Request: []string{"registry", "list"}, EmptyTip: "The registry has no image manifests."},
	}},
	{Path: []string{"alert", "list"}, Summary: "List active alerts"},
	{Path: []string{"alert", "resolve"}, Summary: "Resolve an alert", Confirm: true, Arguments: []Argument{resource("ALERT", SourceAlerts, []string{"alert", "list"}, "There are no active alerts.")}},
	{Path: []string{"help"}, Summary: "Explain every workspace command"},
	{Path: []string{"clear"}, Summary: "Clear the response transcript", Action: ActionClear},
	{Path: []string{"exit"}, Summary: "Close the workspace", Action: ActionExit},
}

func resource(name string, source Source, request []string, empty string) Argument {
	return Argument{Name: name, Label: name, Source: source, Request: request, EmptyTip: empty}
}

func project() Argument {
	return resource("PROJECT", SourceProjects, []string{"project", "list"}, "No loaded projects. Add YAML, then run project reload.")
}

func release() Argument {
	return resource("RELEASE", SourceReleases, []string{"release", "list", "--limit", "100"}, "No releases have been recorded yet.")
}

func runMode() Argument {
	return Argument{Name: "MODE", Label: "RUN MODE", Usage: "[--no-follow]", Optional: true, Choices: []Choice{
		{Display: "follow live", Summary: "Stream the release until it finishes"},
		{Arguments: []string{"--no-follow"}, Display: "background", Summary: "Start the release and return immediately"},
	}}
}

func logMode() Argument {
	return Argument{Name: "MODE", Label: "LOG MODE", Usage: "[--follow]", Optional: true, Choices: []Choice{
		{Display: "recent", Summary: "Show the available release output"},
		{Arguments: []string{"--follow"}, Display: "follow live", Summary: "Keep streaming new release output"},
	}}
}

func Visible() []Command {
	result := make([]Command, 0, len(commands))
	for _, command := range commands {
		if !command.Hidden {
			result = append(result, command)
		}
	}
	return result
}

func Resolve(args []string) (Command, error) {
	if len(args) == 0 {
		return findPath([]string{"help"}), nil
	}
	var matched *Command
	for index := range commands {
		command := &commands[index]
		if hasPrefix(args, command.Path) && (matched == nil || len(command.Path) > len(matched.Path)) {
			matched = command
		}
	}
	if matched == nil {
		return Command{}, groupError(args)
	}
	if !validArguments(matched.Arguments, args[len(matched.Path):]) {
		return Command{}, fmt.Errorf("usage: %s", Usage(*matched))
	}
	return *matched, nil
}

func Find(path ...string) (Command, bool) {
	for _, command := range commands {
		if equal(command.Path, path) {
			return command, true
		}
	}
	return Command{}, false
}

func Usage(command Command) string {
	parts := append([]string{}, command.Path...)
	for _, argument := range command.Arguments {
		if argument.Usage != "" {
			parts = append(parts, argument.Usage)
			continue
		}
		value := argument.Name
		if argument.Optional {
			value = "[" + value + "]"
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, " ")
}

func UsageError(path ...string) error {
	command, found := Find(path...)
	if !found {
		return groupError(path)
	}
	return fmt.Errorf("usage: %s", Usage(command))
}

func GroupUsageError(root string) error {
	return groupError([]string{root})
}

func Path(command Command) string {
	return strings.Join(command.Path, " ")
}

func findPath(path []string) Command {
	command, _ := Find(path...)
	return command
}

func groupError(args []string) error {
	root := args[0]
	usages := make([]string, 0)
	for _, command := range commands {
		if command.Hidden || len(command.Path) == 0 || command.Path[0] != root {
			continue
		}
		usages = append(usages, Usage(command))
	}
	if len(usages) == 0 {
		return fmt.Errorf("unknown command %q; run help", root)
	}
	return fmt.Errorf("usage: %s", strings.Join(usages, " | "))
}

func validArguments(definitions []Argument, values []string) bool {
	offset := 0
	for index, definition := range definitions {
		if offset == len(values) && definition.Optional {
			continue
		}
		if !hasPrefix(values[offset:], definition.Prefix) {
			return false
		}
		offset += len(definition.Prefix)
		remaining := values[offset:]
		if definition.Validation == ValidationContainerLog {
			return index == len(definitions)-1 && validContainerLog(remaining)
		}
		width := max(1, definition.Width)
		if len(remaining) < width {
			return false
		}
		selection := remaining[:width]
		if len(definition.Choices) > 0 && !validChoice(definition.Choices, selection) {
			return false
		}
		if !validValue(definition.Validation, selection) {
			return false
		}
		offset += width
	}
	return offset == len(values)
}

func validChoice(choices []Choice, values []string) bool {
	for _, choice := range choices {
		if equal(choice.Arguments, values) {
			return true
		}
	}
	return false
}

func validValue(validation Validation, values []string) bool {
	if validation == "" {
		return true
	}
	switch validation {
	case ValidationUint:
		_, err := strconv.ParseUint(values[0], 10, 64)
		return err == nil
	case ValidationLimit:
		value, err := strconv.Atoi(values[0])
		return err == nil && value >= 1 && value <= 1000
	default:
		return false
	}
}

func validContainerLog(values []string) bool {
	if len(values) == 0 {
		return true
	}
	follow, tail := false, false
	for index := 0; index < len(values); index++ {
		switch values[index] {
		case "--follow":
			if follow {
				return false
			}
			follow = true
		case "--tail":
			if tail || index+1 >= len(values) {
				return false
			}
			count, err := strconv.Atoi(values[index+1])
			if err != nil || count < 0 || count > 10000 {
				return false
			}
			tail = true
			index++
		default:
			return false
		}
	}
	return follow || tail
}

func hasPrefix(values, prefix []string) bool {
	return len(values) >= len(prefix) && equal(values[:len(prefix)], prefix)
}

func equal(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
