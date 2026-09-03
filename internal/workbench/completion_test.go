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
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/mustafanass/uruflow/internal/grammar"
	"github.com/mustafanass/uruflow/internal/ops"
)

func TestCommandSuggestionsFilterAsTheUserTypes(t *testing.T) {
	matches := matchingCommands("s", 0)
	if len(matches) == 0 || matches[0].Command != "status" {
		t.Fatalf("matches = %#v", matches)
	}
	project := matchingCommands("project dep", 0)
	if len(project) != 1 || project[0].Command != "project deploy" {
		t.Fatalf("project matches = %#v", project)
	}
	if got := matchingCommands("project deploy api-prod", 0); len(got) != 0 {
		t.Fatalf("suggestions remained visible while entering arguments: %#v", got)
	}
	if got := matchingCommands("agent add ", 0); len(got) != 0 {
		t.Fatalf("command suggestion repeated after its argument began: %#v", got)
	}
}

func TestDeployAdvancesFromCommandToLoadedProjects(t *testing.T) {
	completion, ok := argumentCompletion("project deploy ")
	if !ok || completion.Label != "PROJECT" || completion.Query != "" || !completion.NeedsNext {
		t.Fatalf("completion = %#v, %v", completion, ok)
	}
	if completion.Trail != "DEPLOY › PROJECT" {
		t.Fatalf("project trail = %q", completion.Trail)
	}
	items := sourceProjector(completion.Source)(completion.Prefix, ops.Table("projects",
		[]string{"NAME", "ENV", "WORKFLOW", "SERVICES", "SOURCE"}, [][]string{
			{"api-prod", "prod", "build_deploy", "2", "projects/api/prod.yaml"},
			{"web-prod", "prod", "deploy_only", "1", "projects/web/prod.yaml"},
		}))
	input := textinput.New()
	input.SetValue("project deploy api")
	m := &model{input: input, completionCache: map[string][]commandSpec{completion.Key: items}}
	matches := m.suggestions()
	if len(matches) != 1 || matches[0].Command != "project deploy api-prod" {
		t.Fatalf("project matches = %#v", matches)
	}
	mode, ok := argumentCompletion("project deploy api-prod ")
	if !ok || mode.Label != "RUN MODE" || len(mode.Items) != 2 || mode.Items[0].Display != "follow live" {
		t.Fatalf("run mode = %#v, %v", mode, ok)
	}
	if mode.Trail != "DEPLOY › PROJECT › MODE" {
		t.Fatalf("mode trail = %q", mode.Trail)
	}
}

func TestCreationCommandsAdvanceToGuidedInput(t *testing.T) {
	for _, test := range []struct {
		value string
		label string
	}{
		{"agent add ", "NEW AGENT"},
		{"project variables ", "PROJECT"},
		{"project apply ", "PROJECT ENVIRONMENT"},
		{"project validate ", "YAML FILE"},
		{"project create ", "NEW PROJECT"},
	} {
		completion, ok := argumentCompletion(test.value)
		if !ok || completion.Label != test.label || completion.EmptyTip == "" {
			t.Fatalf("%q completion = %#v, %v", test.value, completion, ok)
		}
	}
}

func TestProjectCreateAdvancesFromNameToEnvironment(t *testing.T) {
	completion, ok := argumentCompletion("project create api")
	if !ok || completion.Label != "ENVIRONMENT" || completion.Trail != "CREATE › PROJECT › ENVIRONMENT" {
		t.Fatalf("completion = %#v, %v", completion, ok)
	}
	completion, ok = argumentCompletion("project create api prod")
	if !ok || !completion.AllowRaw {
		t.Fatalf("final creation stage = %#v, %v", completion, ok)
	}
}

func TestContainerLogsSelectsAgentAndContainerTogether(t *testing.T) {
	completion, ok := argumentCompletion("container logs ")
	if !ok || completion.Label != "CONTAINER" || !completion.NeedsNext {
		t.Fatalf("container completion = %#v, %v", completion, ok)
	}
	event := ops.Table("containers",
		[]string{"AGENT", "NAME", "TYPE", "PROJECT", "SERVICE", "ID", "STATE", "HEALTH", "CPU", "MEMORY"},
		[][]string{{"urufi-builder", "registry", "system", "", "", "e47bde10bad4", "running", "none", "0%", "32.5 MB"}})
	items := sourceProjector(completion.Source)(completion.Prefix, event)
	if len(items) != 1 || items[0].Command != "container logs urufi-builder e47bde10bad4" || items[0].Display != "registry · e47bde10bad4" {
		t.Fatalf("container choices = %#v", items)
	}

	incomplete, ok := argumentCompletion("container logs urufi-builder")
	if !ok || incomplete.AllowRaw {
		t.Fatalf("partial container command should stay guided: %#v, %v", incomplete, ok)
	}
	mode, ok := argumentCompletion(items[0].Command + " ")
	if !ok || mode.Label != "LOG MODE" || len(mode.Items) != 4 {
		t.Fatalf("container log mode = %#v, %v", mode, ok)
	}
	if mode.Items[1].Command != items[0].Command+" --follow" || mode.Items[2].Command != items[0].Command+" --tail 100" {
		t.Fatalf("container log modes = %#v", mode.Items)
	}
	custom, ok := argumentCompletion(items[0].Command + " --tail 273 --follow")
	if !ok || !custom.AllowRaw {
		t.Fatalf("custom container tail should remain executable: %#v, %v", custom, ok)
	}
	invalid, ok := argumentCompletion(items[0].Command + " --tail ")
	if !ok || invalid.AllowRaw || len(invalid.Items) == 0 {
		t.Fatalf("incomplete tail should stay in the mode picker: %#v, %v", invalid, ok)
	}
}

func TestCompoundResourceChoicesUseNamedColumns(t *testing.T) {
	registry, ok := argumentCompletion("registry remove ")
	if !ok {
		t.Fatal("registry completion is missing")
	}
	items := sourceProjector(registry.Source)(registry.Prefix, ops.Table("registry",
		[]string{"REPOSITORY", "TAG", "DIGEST", "SIZE", "AGE"},
		[][]string{{"uruflow/api", "2.3.1", "sha256:abc", "42 MB", "now"}}))
	if len(items) != 1 || items[0].Command != "registry remove uruflow/api 2.3.1" || items[0].Display != "uruflow/api:2.3.1" {
		t.Fatalf("registry choices = %#v", items)
	}

	apply, ok := argumentCompletion("project apply ")
	if !ok {
		t.Fatal("project apply completion is missing")
	}
	items = sourceProjector(apply.Source)(apply.Prefix, ops.Table("projects",
		[]string{"NAME", "ENV", "WORKFLOW", "SERVICES", "SOURCE"},
		[][]string{{"api-prod", "prod", "build_deploy", "2", "projects/api/prod.yaml"}}))
	if len(items) != 1 || items[0].Command != "project apply api prod" {
		t.Fatalf("project environment choices = %#v", items)
	}
	file, ok := argumentCompletion(items[0].Command + " ")
	if !ok || file.Label != "YAML FILE" || file.NeedsNext || file.AllowRaw {
		t.Fatalf("project file stage = %#v, %v", file, ok)
	}
	file, ok = argumentCompletion(items[0].Command + " -")
	if !ok || !file.AllowRaw {
		t.Fatalf("project YAML input = %#v, %v", file, ok)
	}
}

func TestEveryArgumentCommandHasAGuidedFlow(t *testing.T) {
	for _, command := range grammar.Visible() {
		if len(command.Arguments) == 0 {
			continue
		}
		completion, ok := argumentCompletion(grammar.Path(command) + " ")
		if !ok || completion.Label == "" {
			t.Errorf("%q has no guided completion", grammar.Path(command))
		}
	}
}

func TestAgentEnrollmentAdvancesFromNameToRole(t *testing.T) {
	input := textinput.New()
	input.SetValue("agent add build-01")
	m := &model{input: input}

	completion, ok := argumentCompletion(input.Value())
	if !ok || completion.Label != "AGENT ROLE" || len(completion.Items) != 3 {
		t.Fatalf("completion = %#v, %v", completion, ok)
	}
	matches := m.suggestions()
	if len(matches) != 3 {
		t.Fatalf("role suggestions = %#v", matches)
	}
	if matches[0].Command != "agent add build-01 --roles runner" || matches[2].Command != "agent add build-01 --roles builder,runner" {
		t.Fatalf("role suggestions = %#v", matches)
	}
	m.input.SetValue("agent add build-01 ")
	if matches = m.suggestions(); len(matches) != 3 {
		t.Fatalf("role suggestions after space = %#v", matches)
	}

	m.input.SetValue("agent add build-01 --roles run")
	matches = m.suggestions()
	if len(matches) != 2 || matches[0].Command != "agent add build-01 --roles runner" || matches[1].Command != "agent add build-01 --roles builder,runner" {
		t.Fatalf("filtered role suggestions = %#v", matches)
	}

	completion, ok = argumentCompletion("agent add build-01 runner ")
	if !ok || completion.Label != "AGENT ROLE" {
		t.Fatalf("role without --roles escaped the guided stage: %#v, %v", completion, ok)
	}
}

func TestAgentRoleSuggestionsRenderOnlyTheRoleValue(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("agent add build-01")
	m := &model{input: input, width: 80}
	output := m.renderCommandArea()
	if !strings.Contains(output, "builder,runner") {
		t.Fatalf("combined role is not visible:\n%s", output)
	}
	if strings.Count(output, "agent add build-01 --roles") != 0 {
		t.Fatalf("role picker repeats the full command:\n%s", output)
	}
}

func TestAdvancingSelectionSaysEnterContinue(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("project deploy ")
	completion, _ := argumentCompletion(input.Value())
	m := &model{
		input: input, width: 90, height: 24,
		completionCache: map[string][]commandSpec{completion.Key: {
			{Command: "project deploy api-prod", Display: "api-prod", Summary: "build_deploy", NeedsArgs: true},
		}},
	}
	output := m.renderCommandArea()
	if !strings.Contains(output, "DEPLOY › PROJECT") || !strings.Contains(output, "Enter continue") || strings.Contains(output, "Enter run") {
		t.Fatalf("advancing hint is unclear:\n%s", output)
	}
}

func TestCompletionFailureIsVisibleAndRetryable(t *testing.T) {
	input := textinput.New()
	input.Prompt = "› "
	input.SetValue("project deploy ")
	completion, _ := argumentCompletion(input.Value())
	m := &model{
		input: input, width: 100, height: 24,
		completionErrors: map[string]string{completion.Key: "control socket unavailable"},
	}
	output := m.renderCommandArea()
	if !strings.Contains(output, "Could not load project") || !strings.Contains(output, "Enter retries") || !strings.Contains(output, "Enter retry") {
		t.Fatalf("completion error is not actionable:\n%s", output)
	}
}

func TestSlashOpensTheWholeCommandCatalog(t *testing.T) {
	if got := len(matchingCommands("/", 0)); got != len(commandCatalog) {
		t.Fatalf("slash returned %d commands, want %d", got, len(commandCatalog))
	}
	if got := len(matchingCommands("show", 0)); got == 0 || got == len(commandCatalog) {
		t.Fatalf("show should filter canonical commands, got %d", got)
	}
}
