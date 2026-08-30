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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/mustafanass/uruflow/internal/cliui"
	"github.com/mustafanass/uruflow/internal/control"
	"github.com/mustafanass/uruflow/internal/ops"
)

type streamMsg struct {
	event *ops.Event
	err   error
	done  bool
}

type editPathMsg struct {
	path string
	err  error
}

type editorDoneMsg struct{ err error }

const (
	maxTranscriptLines  = 5000
	keptTranscriptLines = 4000
)

type model struct {
	client            *control.Client
	viewport          viewport.Model
	input             textinput.Model
	editor            textarea.Model
	events            chan streamMsg
	cancel            context.CancelFunc
	history           []string
	transcript        string
	historyAt         int
	width             int
	height            int
	editorRows        int
	running           bool
	paste             bool
	secret            bool
	confirm           bool
	pending           []string
	active            []string
	suggestionAt      int
	completionCache   map[string][]commandSpec
	completionLoading map[string]bool
	color             bool
	initialized       bool
	pristine          bool
}

func Run(socket string, noColor bool) error {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return errors.New("the URUFLOW workspace requires an interactive terminal")
	}
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "Type a command or / to browse …"
	input.Focus()
	editor := textarea.New()
	editor.Placeholder = "Paste YAML here. Ctrl+S validates and applies; Esc cancels."
	editor.ShowLineNumbers = true
	editor.SetHeight(10)
	editor.SetWidth(80)
	view := viewport.New(80, 20)
	m := &model{
		client: control.NewClient(socket), viewport: view, input: input, editor: editor,
		events: make(chan streamMsg, 64), color: !noColor && os.Getenv("NO_COLOR") == "", pristine: true,
		completionCache: make(map[string][]commandSpec), completionLoading: make(map[string]bool),
	}
	program := tea.NewProgram(m, tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height, m.initialized = msg.Width, msg.Height, true
		m.resize()
		return m, nil
	case streamMsg:
		if msg.event != nil {
			renderer := cliui.New(nil, m.color)
			renderer.Width = max(20, m.width-4)
			m.append(centerResponse(renderer.RenderString(*msg.event)) + "\n")
		}
		if msg.err != nil {
			m.append(m.paint("31", "✘ "+msg.err.Error()) + "\n")
		}
		if msg.done || msg.err != nil {
			m.running = false
			m.cancel = nil
			m.active = nil
			m.completionCache = make(map[string][]commandSpec)
			m.completionLoading = make(map[string]bool)
			m.input.Focus()
			return m, textinput.Blink
		}
		return m, m.waitStream()
	case completionMsg:
		if m.completionCache == nil {
			m.completionCache = make(map[string][]commandSpec)
		}
		if m.completionLoading == nil {
			m.completionLoading = make(map[string]bool)
		}
		delete(m.completionLoading, msg.key)
		if msg.err == nil {
			m.completionCache[msg.key] = msg.items
		}
		m.suggestionAt = 0
		m.resize()
		return m, nil
	case editPathMsg:
		if msg.err != nil {
			m.running = false
			m.input.Focus()
			m.append(m.paint("31", "✘ "+msg.err.Error()) + "\n")
			return m, textinput.Blink
		}
		editor := os.Getenv("VISUAL")
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			editor = "vi"
		}
		parts := strings.Fields(editor)
		process := exec.Command(parts[0], append(parts[1:], msg.path)...)
		return m, tea.ExecProcess(process, func(err error) tea.Msg { return editorDoneMsg{err: err} })
	case editorDoneMsg:
		if msg.err != nil {
			m.running = false
			m.input.Focus()
			m.append(m.paint("31", "✘ editor: "+msg.err.Error()) + "\n")
			return m, textinput.Blink
		}
		m.append(m.paint("32", "✔ editor closed; reloading YAML") + "\n")
		return m, m.start([]string{"project", "reload"}, "")
	case tea.KeyMsg:
		if m.paste {
			return m.updatePaste(msg)
		}
		if m.secret {
			return m.updateSecret(msg)
		}
		if m.confirm {
			return m.updateConfirm(msg)
		}
		switch msg.String() {
		case "ctrl+d":
			if !m.running {
				return m, tea.Quit
			}
		case "ctrl+c":
			if m.running && m.cancel != nil {
				m.cancel()
				message := "▲ detached from live output"
				if isDurableOperation(m.active) {
					message += "; the server-side release continues"
				}
				m.append(m.paint("33", message) + "\n")
				return m, nil
			}
			m.input.SetValue("")
			m.suggestionAt = 0
			m.resize()
			return m, nil
		case "ctrl+l":
			m.transcript = ""
			m.pristine = false
			m.viewport.SetContent("")
			m.input.SetValue("")
			m.suggestionAt = 0
			m.resize()
			return m, nil
		case "esc":
			if m.input.Value() != "" {
				m.input.SetValue("")
				m.suggestionAt = 0
				m.resize()
				return m, nil
			}
		case "tab":
			if m.completeSuggestion() {
				return m, m.requestArgumentCompletion()
			}
		case "shift+tab":
			if suggestions := m.suggestions(); len(suggestions) > 0 {
				m.suggestionAt = (m.suggestionAt - 1 + len(suggestions)) % len(suggestions)
				return m, nil
			}
		case "up":
			if suggestions := m.suggestions(); len(suggestions) > 0 {
				m.suggestionAt = (m.suggestionAt - 1 + len(suggestions)) % len(suggestions)
				return m, nil
			}
			if m.running {
				break
			}
			m.previousHistory()
			m.resize()
			return m, nil
		case "down":
			if suggestions := m.suggestions(); len(suggestions) > 0 {
				m.suggestionAt = (m.suggestionAt + 1) % len(suggestions)
				return m, nil
			}
			if m.running {
				break
			}
			m.nextHistory()
			m.resize()
			return m, nil
		case "enter":
			if m.running {
				return m, nil
			}
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}
			if line == "show" || line == "?" {
				line = "help"
			} else if suggestions := m.suggestions(); len(suggestions) > 0 {
				selected := suggestions[min(m.suggestionAt, len(suggestions)-1)]
				if selected.NeedsArgs {
					m.setCompletion(selected)
					return m, m.requestArgumentCompletion()
				}
				line = selected.Command
			} else if completion, ok := argumentCompletion(m.input.Value()); ok && completion.Query == "" {
				return m, m.requestArgumentCompletion()
			}
			line = strings.TrimSpace(strings.TrimPrefix(line, "/"))
			m.input.SetValue("")
			m.suggestionAt = 0
			m.resize()
			if line == "exit" || line == "quit" {
				return m, tea.Quit
			}
			if line == "clear" {
				m.transcript = ""
				m.pristine = false
				m.viewport.SetContent("")
				return m, nil
			}
			args, err := split(line)
			if err != nil {
				m.append(m.paint("31", "✘ "+err.Error()) + "\n")
				return m, nil
			}
			args = normalize(args)
			m.history = append(m.history, line)
			m.historyAt = len(m.history)
			m.pristine = false
			if isFocusedStream(args) {
				m.transcript = m.streamWelcome()
				m.viewport.SetContent(m.transcript)
				m.viewport.GotoTop()
			}
			m.append(m.paint("36", "› "+line) + "\n\n")
			if wantsPaste(args) {
				m.pending, m.paste = args, true
				m.editor.SetValue("")
				m.editor.Focus()
				m.resize()
				return m, textarea.Blink
			}
			if wantsSecret(args) {
				m.pending, m.secret = args, true
				m.input.SetValue("")
				m.input.Prompt = "Secret › "
				m.input.Placeholder = "value is hidden"
				m.input.EchoMode = textinput.EchoPassword
				m.input.EchoCharacter = '•'
				m.resize()
				return m, textinput.Blink
			}
			if needsConfirmation(args) {
				m.pending, m.confirm = args, true
				m.input.SetValue("")
				m.input.Prompt = "Confirm [y/N] › "
				m.input.Placeholder = strings.Join(args, " ")
				m.resize()
				return m, textinput.Blink
			}
			if len(args) == 3 && args[0] == "project" && args[1] == "edit" {
				m.running = true
				m.input.Blur()
				return m, m.resolveEditor(args[2])
			}
			return m, m.start(args, "")
		}
	}
	if m.paste {
		var command tea.Cmd
		m.editor, command = m.editor.Update(message)
		commands = append(commands, command)
	} else if !m.running {
		var command tea.Cmd
		before := m.input.Value()
		m.input, command = m.input.Update(message)
		if before != m.input.Value() {
			m.suggestionAt = 0
			m.resize()
			commands = append(commands, m.requestArgumentCompletion())
		}
		commands = append(commands, command)
	}
	var viewportCommand tea.Cmd
	m.viewport, viewportCommand = m.viewport.Update(message)
	commands = append(commands, viewportCommand)
	return m, tea.Batch(commands...)
}

func (m *model) resolveEditor(name string) tea.Cmd {
	return func() tea.Msg {
		path := ""
		err := m.client.Execute(context.Background(), []string{"project", "path", name}, "", func(event ops.Event) error {
			if event.Title == "project file" {
				path = event.Message
			}
			return nil
		})
		if err == nil && path == "" {
			err = errors.New("server did not return a project file")
		}
		return editPathMsg{path: path, err: err}
	}
}

func (m *model) updatePaste(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.paste, m.pending = false, nil
		m.editor.Blur()
		m.input.Focus()
		m.append(m.paint("33", "▲ YAML input cancelled") + "\n")
		m.resize()
		return m, textinput.Blink
	case "ctrl+s":
		content := m.editor.Value()
		args := append([]string{}, m.pending...)
		m.paste, m.pending = false, nil
		m.editor.Blur()
		m.resize()
		return m, m.start(args, content)
	}
	var command tea.Cmd
	m.editor, command = m.editor.Update(key)
	return m, command
}

func (m *model) updateSecret(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+c":
		m.resetInputMode()
		m.append(m.paint("33", "▲ secret input cancelled") + "\n")
		return m, textinput.Blink
	case "enter":
		value := m.input.Value()
		args := append([]string{}, m.pending...)
		m.resetInputMode()
		if value == "" {
			m.append(m.paint("31", "✘ secret value cannot be empty") + "\n")
			return m, textinput.Blink
		}
		return m, m.start(args, value)
	}
	var command tea.Cmd
	m.input, command = m.input.Update(key)
	return m, command
}

func (m *model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+c":
		m.resetInputMode()
		m.append(m.paint("33", "▲ action cancelled") + "\n")
		return m, textinput.Blink
	case "enter":
		answer := strings.ToLower(strings.TrimSpace(m.input.Value()))
		args := append([]string{}, m.pending...)
		m.resetInputMode()
		if answer != "y" && answer != "yes" {
			m.append(m.paint("33", "▲ action cancelled") + "\n")
			return m, textinput.Blink
		}
		return m, m.start(args, "")
	}
	var command tea.Cmd
	m.input, command = m.input.Update(key)
	return m, command
}

func (m *model) resetInputMode() {
	m.secret, m.confirm, m.pending = false, false, nil
	m.input.SetValue("")
	m.input.Prompt = "› "
	m.input.Placeholder = "Type a command or / to browse …"
	m.input.EchoMode = textinput.EchoNormal
	m.input.Focus()
	m.resize()
}

func (m *model) start(args []string, input string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel, m.running = cancel, true
	m.active = append([]string{}, args...)
	m.input.Blur()
	go func() {
		err := m.client.Execute(ctx, normalize(args), input, func(event ops.Event) error {
			select {
			case m.events <- streamMsg{event: &event}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if errors.Is(err, context.Canceled) {
			err = nil
		}
		m.events <- streamMsg{err: err, done: true}
	}()
	return m.waitStream()
}

func (m *model) waitStream() tea.Cmd {
	return func() tea.Msg { return <-m.events }
}

func normalize(args []string) []string {
	if len(args) == 0 {
		return args
	}
	switch args[0] {
	case "show", "?":
		return []string{"help"}
	case "deploy", "rollback", "stop":
		return append([]string{"project", args[0]}, args[1:]...)
	case "logs":
		return append([]string{"release", "logs"}, args[1:]...)
	case "edit":
		if len(args) >= 3 && args[1] == "project" {
			return append([]string{"project", "edit"}, args[2:]...)
		}
	}
	return args
}

func wantsPaste(args []string) bool {
	if len(args) == 0 || args[len(args)-1] != "-" {
		return false
	}
	return len(args) >= 2 && args[0] == "project" && (args[1] == "apply" || args[1] == "validate")
}

func wantsSecret(args []string) bool {
	return len(args) == 3 && (args[0] == "secret" || args[0] == "secrets") && args[1] == "set"
}

func needsConfirmation(args []string) bool {
	if len(args) < 2 {
		return false
	}
	command := args[0] + " " + args[1]
	switch command {
	case "agent remove", "agents remove", "project stop", "projects stop", "registry remove", "secret remove", "secrets remove":
		return true
	default:
		return false
	}
}

func isDurableOperation(args []string) bool {
	return len(args) >= 2 && args[0] == "project" && (args[1] == "deploy" || args[1] == "rollback")
}

func isFocusedStream(args []string) bool {
	return len(args) == 1 && args[0] == "events"
}

func (m *model) suggestions() []commandSpec {
	if m.running || m.paste || m.secret || m.confirm {
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

func (m *model) append(value string) {
	follow := !m.initialized || m.viewport.AtBottom()
	m.transcript += value
	lines := strings.Split(m.transcript, "\n")
	if len(lines) > maxTranscriptLines {
		lines = lines[len(lines)-keptTranscriptLines:]
		m.transcript = m.paint("90", "… earlier output omitted …") + "\n" + strings.Join(lines, "\n")
	}
	m.viewport.SetContent(m.transcript)
	if follow {
		m.viewport.GotoBottom()
	}
}

func centerResponse(value string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		if line != "" {
			lines[index] = " " + line
		}
	}
	return strings.Join(lines, "\n")
}

func (m *model) resize() {
	if !m.initialized {
		return
	}
	inputRows := 4
	if m.paste {
		m.editorRows = min(14, max(6, m.height/3))
		inputRows = m.editorRows + 3
		m.editor.SetWidth(max(20, m.width-2))
		m.editor.SetHeight(m.editorRows)
	} else if visible, _ := m.visibleSuggestions(); len(visible) > 0 {
		inputRows += len(visible) + 1
	} else if _, ok := argumentCompletion(m.input.Value()); ok {
		inputRows += 2
	}
	m.viewport.Width = max(20, m.width-2)
	m.viewport.Height = max(4, m.height-inputRows-2)
	m.input.Width = max(10, m.width-8)
	if m.pristine {
		m.transcript = m.welcome()
		m.viewport.SetContent(m.transcript)
		m.viewport.GotoTop()
	}
}

func (m *model) View() string {
	if !m.initialized {
		return ""
	}
	state, shortState, stateColor := "ready", "ready", "32"
	if m.running {
		state, shortState = "streaming · Ctrl+C detaches", "live"
	} else if m.paste {
		state, shortState, stateColor = "editing YAML", "YAML", "33"
	} else if m.secret {
		state, shortState, stateColor = "secure input", "secret", "33"
	} else if m.confirm {
		state, shortState, stateColor = "confirmation", "confirm", "33"
	}
	brand := m.paint("38;5;43", "◆ URUFLOW")
	status := "● " + state
	if utf8Width("◆ URUFLOW")+utf8Width(status)+2 > m.width {
		status = "● " + shortState
	}
	header := brand + strings.Repeat(" ", max(1, m.width-utf8Width("◆ URUFLOW")-utf8Width(status))) + m.paint(stateColor, status)
	rule := m.paint("90", strings.Repeat("─", max(1, m.width)))
	var commandArea string
	if m.paste {
		commandArea = m.renderYAMLArea()
	} else {
		commandArea = m.renderCommandArea()
	}
	return header + "\n" + rule + "\n" + m.viewport.View() + "\n" + commandArea
}

func (m *model) renderCommandArea() string {
	visible, start := m.visibleSuggestions()
	title := " COMMAND "
	completion, completing := argumentCompletion(m.input.Value())
	if completing {
		title = " " + completion.Label + " "
	} else if len(visible) > 0 {
		title = " COMMANDS "
	} else if m.secret {
		title = " SECURE VALUE "
	} else if m.confirm {
		title = " CONFIRM "
	}
	lines := []string{m.frameTop(title)}
	commandWidth := 0
	for _, item := range visible {
		commandWidth = max(commandWidth, utf8Width(suggestionValue(item, completion, completing)))
	}
	commandWidth = min(commandWidth, 24)
	for index, item := range visible {
		selected := start+index == m.suggestionAt
		marker := "  "
		if selected {
			marker = m.paint("38;5;43", "› ")
		}
		command := padText(suggestionValue(item, completion, completing), commandWidth)
		if selected {
			command = m.paint("1;38;5;43", command)
		} else {
			command = m.paint("36", command)
		}
		lines = append(lines, m.frameRow(marker+command+"  "+m.paint("90", item.Summary)))
	}
	if completing && len(visible) == 0 {
		message := "Type a name to continue"
		if len(completion.Items) > 0 && completion.EmptyTip != "" {
			message = completion.EmptyTip
		} else if len(completion.Request) == 0 && completion.EmptyTip != "" {
			message = completion.EmptyTip
		} else if m.completionLoading[completion.Key] {
			message = "Loading " + strings.ToLower(completion.Label) + " …"
		} else if cached, found := m.completionCache[completion.Key]; found && len(cached) == 0 {
			message = completion.EmptyTip
		} else if found && completion.Query != "" {
			message = "No matching " + strings.ToLower(strings.TrimSuffix(completion.Label, "S"))
		}
		lines = append(lines, m.frameRow(m.paint("90", "  "+message)))
	}
	if len(visible) > 0 || completing {
		lines = append(lines, m.frameMiddle())
	}
	lines = append(lines, m.frameRow(m.input.View()), m.frameBottom())

	hint := "  ↑↓ history · / commands · PgUp/PgDn scroll · Ctrl+D exit"
	if len(visible) > 0 {
		hint = "  ↑↓ select · Tab complete · Enter run · Esc close"
	} else if completing {
		hint = "  Type to filter · ↑↓ select · Enter run"
	} else if m.secret || m.confirm {
		hint = "  Enter continue · Esc cancel"
	}
	if m.width < utf8Width(hint) {
		hint = "  / commands · ^D exit"
	}
	lines = append(lines, m.paint("90", hint))
	return strings.Join(lines, "\n")
}

func suggestionValue(item commandSpec, completion argumentContext, completing bool) string {
	if completing && strings.HasPrefix(item.Command, completion.Prefix) {
		return strings.TrimPrefix(item.Command, completion.Prefix)
	}
	return item.Command
}

func (m *model) renderYAMLArea() string {
	return strings.Join([]string{
		m.frameTop(" YAML "),
		m.editor.View(),
		m.frameBottom(),
		m.paint("90", "  Ctrl+S validate and apply · Esc cancel"),
	}, "\n")
}

func (m *model) frameTop(title string) string {
	inner := max(1, m.width-2)
	label := truncateText(title, inner)
	return m.paint("36", "╭"+label+strings.Repeat("─", max(0, inner-utf8Width(label)))+"╮")
}

func (m *model) frameMiddle() string {
	return m.paint("90", "├"+strings.Repeat("─", max(1, m.width-2))+"┤")
}

func (m *model) frameBottom() string {
	return m.paint("36", "╰"+strings.Repeat("─", max(1, m.width-2))+"╯")
}

func (m *model) frameRow(value string) string {
	contentWidth := max(1, m.width-4)
	value = truncateText(value, contentWidth)
	return m.paint("36", "│") + " " + value + strings.Repeat(" ", max(0, contentWidth-utf8Width(value))) + " " + m.paint("36", "│")
}

func (m *model) welcome() string {
	width := min(66, max(20, m.width-6))
	inner := width - 2
	row := func(value string) string {
		value = truncateText(value, inner-4)
		return m.paint("36", "│") + "  " + value + strings.Repeat(" ", max(0, inner-4-utf8Width(value))) + "  " + m.paint("36", "│")
	}
	lines := []string{
		m.paint("36", "╭"+strings.Repeat("─", inner)+"╮"),
		row(""),
		row(m.paint("1;38;5;43", "◆  Welcome to URUFLOW")),
		row(m.paint("90", "One workspace for your whole fleet.")),
		row(""),
		row(m.paint("36", "status") + "     Fleet health and active work"),
		row(m.paint("36", "events") + "     Follow live fleet activity"),
		row(m.paint("36", "deploy") + "     Start and follow a release"),
		row(m.paint("36", "/") + "          Browse every command"),
		row(""),
		m.paint("36", "╰"+strings.Repeat("─", inner)+"╯"),
	}
	indent := strings.Repeat(" ", max(0, (m.width-2-width)/2))
	for index := range lines {
		lines[index] = indent + lines[index]
	}
	vertical := strings.Repeat("\n", max(0, (m.viewport.Height-len(lines))/2))
	return vertical + strings.Join(lines, "\n")
}

func (m *model) streamWelcome() string {
	width := min(66, max(20, m.width-4))
	inner := width - 2
	row := func(value string) string {
		value = truncateText(value, inner-4)
		return m.paint("36", "│") + "  " + value + strings.Repeat(" ", max(0, inner-4-utf8Width(value))) + "  " + m.paint("36", "│")
	}
	return strings.Join([]string{
		m.paint("36", "╭"+strings.Repeat("─", inner)+"╮"),
		row(m.paint("1;38;5;43", "●  LIVE FLEET ACTIVITY")),
		row("Releases · build output · state changes · alerts"),
		row(m.paint("90", "Streaming from now · Ctrl+C detaches")),
		m.paint("36", "╰"+strings.Repeat("─", inner)+"╯"),
		"",
	}, "\n")
}

func (m *model) previousHistory() {
	if len(m.history) == 0 {
		return
	}
	m.historyAt = max(0, m.historyAt-1)
	m.input.SetValue(m.history[m.historyAt])
	m.input.CursorEnd()
}

func (m *model) nextHistory() {
	if len(m.history) == 0 {
		return
	}
	m.historyAt = min(len(m.history), m.historyAt+1)
	if m.historyAt == len(m.history) {
		m.input.SetValue("")
		return
	}
	m.input.SetValue(m.history[m.historyAt])
	m.input.CursorEnd()
}

func (m *model) paint(code, value string) string {
	if !m.color {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func utf8Width(value string) int { return lipgloss.Width(value) }

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8Width(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "…")
}

func padText(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-utf8Width(value)))
}

func split(line string) ([]string, error) {
	var args []string
	var word strings.Builder
	quote := rune(0)
	escaped := false
	flush := func() {
		if word.Len() > 0 {
			args = append(args, word.String())
			word.Reset()
		}
	}
	for _, char := range line {
		if escaped {
			word.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				word.WriteRune(char)
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == ' ' || char == '\t' {
			flush()
			continue
		}
		word.WriteRune(char)
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("unfinished quote or escape")
	}
	flush()
	return args, nil
}
