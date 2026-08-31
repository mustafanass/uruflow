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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/mustafanass/uruflow/internal/ops"
)

const defaultWidth = 100

type Renderer struct {
	Writer io.Writer
	Color  bool
	Width  int
}

func New(writer io.Writer, color bool) *Renderer {
	width := defaultWidth
	if file, ok := writer.(*os.File); ok {
		if measured, _, err := term.GetSize(file.Fd()); err == nil && measured > 0 {
			width = measured
		}
	}
	return &Renderer{Writer: writer, Color: color, Width: width}
}

func (r *Renderer) Render(event ops.Event) error {
	event = safeEvent(event)
	var output string
	switch event.Type {
	case ops.EventTable:
		output = r.table(event.Title, event.Columns, event.Rows)
	case ops.EventLog:
		output = r.log(event)
	case ops.EventResult:
		output = r.result(event)
	default:
		output = r.message(event)
	}
	_, err := fmt.Fprintln(r.Writer, output)
	return err
}

func (r *Renderer) RenderString(event ops.Event) string {
	builder := &strings.Builder{}
	copy := *r
	copy.Writer = builder
	_ = copy.Render(event)
	return strings.TrimSuffix(builder.String(), "\n")
}

func (r *Renderer) message(event ops.Event) string {
	symbol, color := "•", ANSIInformation
	switch event.Level {
	case "success":
		symbol, color = "✔", ANSISuccess
	case "warning":
		symbol, color = "▲", ANSIWarning
	case "error":
		symbol, color = "✘", ANSIError
	}
	prefix := ""
	if event.Operation != "" {
		prefix = r.paint(ANSIMuted, event.Time.Local().Format("15:04:05")) + "  "
	}
	return prefix + r.paint(color, symbol) + " " + event.Message
}

func (r *Renderer) log(event ops.Event) string {
	stamp := event.Time.Local().Format("15:04:05")
	source := event.Title
	if source == "" {
		source = "server"
	}
	color := ANSIInformation
	if event.Level == "warning" {
		color = ANSIWarning
	} else if event.Level == "error" {
		color = ANSIError
	}
	return r.paint(ANSIMuted, stamp) + "  " + r.paint(color, pad(truncate(source, 14), 14)) + "  " + event.Message
}

func (r *Renderer) result(event ops.Event) string {
	values, ok := event.Data.(map[string]any)
	if !ok {
		payload, _ := json.MarshalIndent(event.Data, "", "  ")
		return r.panel(event.Title, strings.Split(string(payload), "\n"))
	}
	if event.Title == "fleet" {
		return r.fleet(values)
	}
	if event.Title == "agent enrolled" {
		return r.agentEnrollment(values)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]string, 0, len(keys))
	for _, key := range keys {
		if nested, nestedOK := values[key].(map[string]any); nestedOK {
			payload, _ := json.Marshal(nested)
			rows = append(rows, fmt.Sprintf("%-18s %s", key, payload))
			continue
		}
		rows = append(rows, fmt.Sprintf("%-18s %v", key, values[key]))
	}
	return r.panel(event.Title, rows)
}

func (r *Renderer) agentEnrollment(values map[string]any) string {
	name := fmt.Sprint(values["name"])
	roles := fmt.Sprint(values["roles"])
	rows := []string{
		name + "  " + r.paint(ANSISuccess, strings.ToUpper(roles)),
		"",
		"Run once on the target:",
		"  uruflow-agent init \\",
		fmt.Sprintf("    --id %v \\", values["id"]),
		fmt.Sprintf("    --key %v \\", values["key"]),
		fmt.Sprintf("    --server %v \\", values["server"]),
		fmt.Sprintf("    --roles %s", roles),
		"",
		"Trust root on the server:",
		"  " + fmt.Sprint(values["ca_certificate"]),
		"Copy it to the agent CA path before starting the service.",
		r.paint(ANSIWarning, "The key is only available in this enrollment response."),
	}
	return r.panel("agent enrolled", rows)
}

func (r *Renderer) fleet(values map[string]any) string {
	agents := fmt.Sprintf("%v/%v online", values["agents_online"], values["agents_total"])
	rows := []string{
		fmt.Sprintf("%-12s %-20s %-14s %v", "Agents", agents, "Containers", values["containers_running"]),
		fmt.Sprintf("%-12s %-20v %-14s %v", "Projects", values["projects"], "Active releases", values["releases_active"]),
		fmt.Sprintf("%-12s %-20v %-14s %v", "Registry", values["registry"], "Alerts", values["alerts"]),
	}
	return r.panel("fleet", rows)
}

func (r *Renderer) panel(title string, rows []string) string {
	width := max(16, r.Width)
	inner := width - 2
	titleText := " " + truncate(strings.ToUpper(title), max(1, inner-2)) + " "
	lines := []string{r.paint(ANSIBorder, "╭") + r.paint(ANSIAccentBold, titleText) + r.paint(ANSIBorder, strings.Repeat("─", max(0, inner-displayWidth(titleText)))+"╮")}
	for _, row := range rows {
		lines = append(lines, r.paint(ANSIBorder, "│")+" "+pad(truncate(row, inner-2), inner-2)+" "+r.paint(ANSIBorder, "│"))
	}
	lines = append(lines, r.paint(ANSIBorder, "╰"+strings.Repeat("─", inner)+"╯"))
	return strings.Join(lines, "\n")
}

func (r *Renderer) table(title string, columns []string, rows [][]string) string {
	if len(columns) == 0 {
		return r.panel(title, []string{"no data"})
	}
	widths := make([]int, len(columns))
	for index, column := range columns {
		widths[index] = displayWidth(column)
	}
	for _, row := range rows {
		for index := range widths {
			if index < len(row) {
				widths[index] = min(max(widths[index], displayWidth(row[index])), 40)
			}
		}
	}
	available := max(12, max(16, r.Width)-4)
	for tableWidth(widths) > available {
		largest := 0
		for index := range widths {
			if widths[index] > widths[largest] {
				largest = index
			}
		}
		if widths[largest] <= 4 {
			break
		}
		widths[largest]--
	}
	for len(widths) > 1 && tableWidth(widths) > available {
		widths = widths[:len(widths)-1]
	}
	expandTable(widths, available)
	line := func(values []string, heading bool) string {
		cells := make([]string, len(widths))
		for index, width := range widths {
			value := ""
			if index < len(values) {
				value = values[index]
			}
			value = pad(truncate(value, width), width)
			if heading {
				value = r.paint(ANSIAccentBold, value)
			} else if strings.EqualFold(value, pad("online", width)) || strings.EqualFold(value, pad("succeeded", width)) || strings.EqualFold(value, pad("healthy", width)) {
				value = r.paint(ANSISuccess, value)
			} else if strings.EqualFold(value, pad("failed", width)) || strings.EqualFold(value, pad("offline", width)) {
				value = r.paint(ANSIError, value)
			}
			cells[index] = value
		}
		return strings.Join(cells, "  ")
	}
	content := []string{line(columns, true)}
	if len(rows) == 0 {
		content = append(content, r.paint(ANSIMuted, center("no records", tableWidth(widths))))
	} else {
		for _, row := range rows {
			content = append(content, line(row, false))
		}
	}
	return r.panel(title, content)
}

func (r *Renderer) paint(code, value string) string {
	return Paint(r.Color, code, value)
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return ansi.Truncate(value, width, "…")
}

func pad(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-displayWidth(value)))
}

func displayWidth(value string) int {
	return ansi.StringWidth(value)
}

func total(values []int) int {
	result := 0
	for _, value := range values {
		result += value
	}
	return result
}

func tableWidth(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return total(values) + (len(values)-1)*2
}

func expandTable(widths []int, available int) {
	if len(widths) == 0 {
		return
	}
	extra := max(0, available-tableWidth(widths))
	for index := range widths {
		widths[index] += extra / len(widths)
		if index < extra%len(widths) {
			widths[index]++
		}
	}
}

func center(value string, width int) string {
	remaining := max(0, width-displayWidth(value))
	left := remaining / 2
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", remaining-left)
}

func Header(writer io.Writer, color bool, state string) {
	renderer := New(writer, color)
	brand := Wordmark(color)
	status := renderer.paint(ANSISuccess, "● "+state)
	fmt.Fprintf(writer, "%s  %s  %s\n", brand, renderer.paint(ANSIMuted, time.Now().Format("2006-01-02 15:04")), status)
}
