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

package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/tui/theme"
)

const (
	labelWidth  = 16
	optionGap   = "  "
	valueYes    = "yes"
	valueNo     = "no"
	emptyChoice = "none available"
)

type FieldKind int

const (
	FieldText FieldKind = iota
	FieldChoice
	FieldMulti
	FieldToggle
)

type Option struct {
	Value string
	Label string
}

type Field struct {
	Label    string
	Hint     string
	Optional bool

	kind     FieldKind
	input    textinput.Model
	options  []Option
	selected map[string]bool
	cursor   int
}

type Form struct {
	Fields []*Field
	Cursor int
}

func NewField(label, placeholder, hint string, optional bool) *Field {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Prompt = ""
	input.PlaceholderStyle = theme.Ghost
	input.TextStyle = theme.Body
	input.Cursor.Style = theme.Lead

	return &Field{Label: label, Hint: hint, Optional: optional, kind: FieldText, input: input}
}

func NewChoice(label, hint string, optional bool) *Field {
	return &Field{Label: label, Hint: hint, Optional: optional, kind: FieldChoice}
}

func NewMulti(label, hint string, optional bool) *Field {
	return &Field{Label: label, Hint: hint, Optional: optional, kind: FieldMulti,
		selected: make(map[string]bool)}
}

func NewToggle(label, hint string) *Field {
	field := &Field{Label: label, Hint: hint, Optional: true, kind: FieldToggle}
	field.options = []Option{{Value: valueYes, Label: valueYes}, {Value: valueNo, Label: valueNo}}
	return field
}

func NewForm(fields ...*Field) *Form {
	form := &Form{Fields: fields}
	form.focus()
	return form
}

func (f *Field) Secret() {
	f.input.EchoMode = textinput.EchoPassword
	f.input.EchoCharacter = '•'
}

func (f *Field) SetOptions(options []Option) {
	f.options = options
	if f.cursor >= len(options) {
		f.cursor = 0
	}
	if f.kind == FieldMulti {
		for value := range f.selected {
			if !f.hasOption(value) {
				delete(f.selected, value)
			}
		}
	}
}

func (f *Field) hasOption(value string) bool {
	for _, option := range f.options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func (f *Field) Value() string {
	switch f.kind {
	case FieldChoice, FieldToggle:
		if f.cursor < len(f.options) {
			return f.options[f.cursor].Value
		}
		return ""
	case FieldMulti:
		return strings.Join(f.Values(), ",")
	default:
		return strings.TrimSpace(f.input.Value())
	}
}

func (f *Field) Values() []string {
	values := make([]string, 0, len(f.selected))
	for _, option := range f.options {
		if f.selected[option.Value] {
			values = append(values, option.Value)
		}
	}
	return values
}

func (f *Field) Set(value string) {
	switch f.kind {
	case FieldChoice, FieldToggle:
		for index, option := range f.options {
			if option.Value == value {
				f.cursor = index
				return
			}
		}
		f.cursor = 0
	case FieldMulti:
		f.SetValues(strings.Split(value, ","))
	default:
		f.input.SetValue(value)
	}
}

func (f *Field) SetValues(values []string) {
	f.selected = make(map[string]bool, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			f.selected[trimmed] = true
		}
	}
}

func (f *Field) Reset() {
	f.cursor = 0
	f.selected = make(map[string]bool)
	f.input.SetValue("")
}

func (f *Field) empty() bool {
	switch f.kind {
	case FieldMulti:
		return len(f.Values()) == 0
	default:
		return f.Value() == ""
	}
}

func (f *Field) update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		if f.kind == FieldText {
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			return cmd
		}
		return nil
	}

	switch f.kind {
	case FieldChoice, FieldToggle:
		switch key.String() {
		case "left", "h":
			f.step(-1)
		case "right", "l", " ":
			f.step(1)
		}
		return nil

	case FieldMulti:
		switch key.String() {
		case "left", "h":
			f.move(-1)
		case "right", "l":
			f.move(1)
		case " ":
			if f.cursor < len(f.options) {
				value := f.options[f.cursor].Value
				if f.selected[value] {
					delete(f.selected, value)
				} else {
					f.selected[value] = true
				}
			}
		}
		return nil

	default:
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return cmd
	}
}

func (f *Field) step(delta int) {
	if len(f.options) == 0 {
		return
	}
	f.cursor = (f.cursor + delta + len(f.options)) % len(f.options)
}

func (f *Field) move(delta int) {
	if len(f.options) == 0 {
		return
	}
	f.cursor = (f.cursor + delta + len(f.options)) % len(f.options)
}

func (f *Form) Field(index int) *Field {
	if index < 0 || index >= len(f.Fields) {
		return nil
	}
	return f.Fields[index]
}

func (f *Form) Value(index int) string {
	if field := f.Field(index); field != nil {
		return field.Value()
	}
	return ""
}

func (f *Form) Values(index int) []string {
	if field := f.Field(index); field != nil {
		return field.Values()
	}
	return nil
}

func (f *Form) Reset() {
	for _, field := range f.Fields {
		field.Reset()
	}
	f.Cursor = 0
	f.focus()
}

func (f *Form) Next() {
	f.Cursor = (f.Cursor + 1) % len(f.Fields)
	f.focus()
}

func (f *Form) Previous() {
	f.Cursor--
	if f.Cursor < 0 {
		f.Cursor = len(f.Fields) - 1
	}
	f.focus()
}

func (f *Form) Update(msg tea.Msg) tea.Cmd {
	return f.Fields[f.Cursor].update(msg)
}

func (f *Form) Missing() string {
	for _, field := range f.Fields {
		if !field.Optional && field.empty() {
			return field.Label
		}
	}
	return ""
}

func (f *Form) Render(width int) string {
	inner := width - labelWidth
	if inner < 12 {
		inner = 12
	}

	lines := make([]string, 0, len(f.Fields)*3)
	for index, field := range f.Fields {
		focused := index == f.Cursor

		label := theme.Ghost.Render(theme.Cell(field.Label, labelWidth))
		rule := theme.Rule
		if focused {
			label = theme.Lead.Render(theme.Cell(theme.IconPointer+" "+field.Label, labelWidth))
			rule = theme.Lead
		}

		lines = append(lines,
			label+theme.Cell(field.render(inner, focused), inner),
			rule.Render(strings.Repeat(theme.IconLine, width)))

		if focused && field.hint() != "" {
			lines = append(lines, strings.Repeat(" ", labelWidth)+theme.Ghost.Render(field.hint()))
		}
	}

	return strings.Join(lines, "\n")
}

func (f *Field) hint() string {
	switch f.kind {
	case FieldChoice, FieldToggle:
		return theme.IconLeft + " " + theme.IconRight + " to change" + suffix(f.Hint)
	case FieldMulti:
		return theme.IconLeft + " " + theme.IconRight + " to move, space to toggle" + suffix(f.Hint)
	default:
		return f.Hint
	}
}

func suffix(hint string) string {
	if hint == "" {
		return ""
	}
	return "  " + theme.IconSeparator + "  " + hint
}

func (f *Field) render(width int, focused bool) string {
	switch f.kind {
	case FieldChoice, FieldToggle:
		return f.renderChoice()
	case FieldMulti:
		return f.renderMulti(focused)
	default:
		f.input.Width = width - 2
		return f.input.View()
	}
}

func (f *Field) renderChoice() string {
	if len(f.options) == 0 {
		return theme.Ghost.Render(emptyChoice)
	}

	parts := make([]string, 0, len(f.options))
	for index, option := range f.options {
		if index == f.cursor {
			parts = append(parts, theme.Lead.Render(theme.IconRadioOn+" "+option.Label))
			continue
		}
		parts = append(parts, theme.Ghost.Render(theme.IconRadioOff+" "+option.Label))
	}
	return strings.Join(parts, optionGap)
}

func (f *Field) renderMulti(focused bool) string {
	if len(f.options) == 0 {
		return theme.Ghost.Render(emptyChoice)
	}

	parts := make([]string, 0, len(f.options))
	for index, option := range f.options {
		icon, style := theme.IconUnchecked, theme.Ghost
		if f.selected[option.Value] {
			icon, style = theme.IconChecked, theme.Good
		}

		entry := icon + " " + option.Label
		if focused && index == f.cursor {
			parts = append(parts, theme.Selected.Render(entry))
			continue
		}
		parts = append(parts, style.Render(entry))
	}
	return strings.Join(parts, optionGap)
}

func (f *Form) focus() {
	for index, field := range f.Fields {
		if field.kind != FieldText {
			continue
		}
		if index == f.Cursor {
			field.input.Focus()
			continue
		}
		field.input.Blur()
	}
}
