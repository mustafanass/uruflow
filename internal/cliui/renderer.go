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
	prefix += r.paint(color, symbol) + " "
	return hangingWrap(prefix, event.Message, max(16, r.Width))
}

func (r *Renderer) log(event ops.Event) string {
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
	width := max(16, r.Width)
	sourceWidth := min(14, max(6, width/4))
	prefix := r.paint(color, pad(truncate(source, sourceWidth), sourceWidth)) + "  "
	if width >= 48 {
		prefix = r.paint(ANSIMuted, event.Time.Local().Format("15:04:05")) + "  " + prefix
	}
	return hangingWrap(prefix, event.Message, width)
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
	available := max(12, max(16, r.Width)-4)
	labelWidth := 0
	for _, key := range keys {
		labelWidth = max(labelWidth, displayWidth(key))
	}
	labelWidth = min(labelWidth, min(18, max(4, available/3)))
	rows := make([]string, 0, len(keys))
	for _, key := range keys {
		value := ""
		if nested, nestedOK := values[key].(map[string]any); nestedOK {
			payload, _ := json.Marshal(nested)
			value = string(payload)
		} else {
			value = fmt.Sprint(values[key])
		}
		label := r.paint(ANSIMuted, pad(truncate(key, labelWidth), labelWidth))
		raw := value
		value = truncate(value, max(1, available-labelWidth-2))
		if style := valueStyle(raw); style != "" {
			value = r.paint(style, value)
		}
		rows = append(rows, label+"  "+value)
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
	if len(columns) == 2 && strings.EqualFold(columns[0], "field") && strings.EqualFold(columns[1], "value") {
		return r.details(title, rows)
	}
	available := max(12, max(16, r.Width)-4)
	indices := visibleColumns(columns, rows, available)
	widths := fittedColumnWidths(columns, rows, indices, available)
	hidden := len(columns) - len(indices)
	title = fmt.Sprintf("%s · %d", title, len(rows))
	if hidden > 0 {
		title += fmt.Sprintf(" · +%d", hidden)
	}
	line := func(values []string, heading bool) string {
		cells := make([]string, len(widths))
		for index, width := range widths {
			value := ""
			column := indices[index]
			if column < len(values) {
				value = values[column]
			}
			raw := value
			value = align(truncate(value, width), width, rightAlignedColumn(columns[column]))
			if heading {
				value = r.paint(ANSIAccentBold, value)
			} else if style := valueStyle(raw); style != "" {
				value = r.paint(style, value)
			}
			cells[index] = value
		}
		return strings.Join(cells, "  ")
	}
	content := []string{line(columns, true)}
	if len(rows) == 0 {
		content = append(content, r.paint(ANSIMuted, center("no records", available)))
	} else {
		for _, row := range rows {
			content = append(content, line(row, false))
		}
	}
	return r.panel(title, content)
}

func (r *Renderer) details(title string, rows [][]string) string {
	available := max(12, max(16, r.Width)-4)
	labelWidth := 0
	for _, row := range rows {
		if len(row) > 0 {
			labelWidth = max(labelWidth, displayWidth(row[0]))
		}
	}
	labelWidth = min(labelWidth, min(18, max(4, available/3)))
	content := make([]string, 0, len(rows))
	for _, row := range rows {
		label, value := "", ""
		if len(row) > 0 {
			label = row[0]
		}
		if len(row) > 1 {
			value = row[1]
		}
		label = r.paint(ANSIMuted, pad(truncate(label, labelWidth), labelWidth))
		valueWidth := max(1, available-labelWidth-2)
		raw := value
		value = truncate(value, valueWidth)
		if style := valueStyle(raw); style != "" {
			value = r.paint(style, value)
		}
		content = append(content, label+"  "+value)
	}
	if len(content) == 0 {
		content = append(content, r.paint(ANSIMuted, "no data"))
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

func padLeft(value string, width int) string {
	return strings.Repeat(" ", max(0, width-displayWidth(value))) + value
}

func align(value string, width int, right bool) string {
	if right {
		return padLeft(value, width)
	}
	return pad(value, width)
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

func visibleColumns(columns []string, rows [][]string, available int) []int {
	indices := make([]int, len(columns))
	for index := range columns {
		indices[index] = index
	}
	for len(indices) > 1 {
		if preferredTableWidth(columns, rows, indices) <= available {
			break
		}
		if len(indices) <= 4 && minimumTableWidth(columns, rows, indices) <= available {
			break
		}
		remove := 1
		for index := 2; index < len(indices); index++ {
			if columnPriority(columns[indices[index]], indices[index]) < columnPriority(columns[indices[remove]], indices[remove]) {
				remove = index
			}
		}
		indices = append(indices[:remove], indices[remove+1:]...)
	}
	return indices
}

func preferredTableWidth(columns []string, rows [][]string, indices []int) int {
	widths := make([]int, len(indices))
	for index, column := range indices {
		widths[index] = preferredColumnWidth(columns[column], rows, column)
	}
	return tableWidth(widths)
}

func minimumTableWidth(columns []string, rows [][]string, indices []int) int {
	widths := make([]int, len(indices))
	for index, column := range indices {
		widths[index] = minimumColumnWidth(columns[column], rows, column)
	}
	return tableWidth(widths)
}

func minimumColumnWidth(header string, rows [][]string, column int) int {
	desired := columnContentWidth(header, rows, column)
	return min(desired, max(3, min(6, displayWidth(header))))
}

func preferredColumnWidth(header string, rows [][]string, column int) int {
	desired := columnContentWidth(header, rows, column)
	width := 6
	switch strings.ToUpper(strings.TrimSpace(header)) {
	case "NAME":
		width = 12
	case "STATE", "STATUS":
		width = 10
	case "HEALTH":
		width = 9
	case "MESSAGE", "DETAIL":
		width = 16
	case "PROJECT", "SERVICE", "AGENT", "SOURCE":
		width = 8
	}
	return min(desired, width)
}

func columnContentWidth(header string, rows [][]string, column int) int {
	width := displayWidth(header)
	for _, row := range rows {
		if column < len(row) {
			width = max(width, displayWidth(row[column]))
		}
	}
	return width
}

func fittedColumnWidths(columns []string, rows [][]string, indices []int, available int) []int {
	widths := make([]int, len(indices))
	desired := make([]int, len(indices))
	minimum := make([]int, len(indices))
	for index, column := range indices {
		desired[index] = columnContentWidth(columns[column], rows, column)
		minimum[index] = minimumColumnWidth(columns[column], rows, column)
		widths[index] = min(desired[index], 40)
	}
	for tableWidth(widths) > available {
		shrink := -1
		for index := range widths {
			if widths[index] <= minimum[index] {
				continue
			}
			if shrink < 0 || widths[index]-minimum[index] > widths[shrink]-minimum[shrink] {
				shrink = index
			}
		}
		if shrink < 0 {
			break
		}
		widths[shrink]--
	}
	for tableWidth(widths) < available {
		grow := -1
		for index := range widths {
			if widths[index] >= desired[index] {
				continue
			}
			if grow < 0 || desired[index]-widths[index] > desired[grow]-widths[grow] {
				grow = index
			}
		}
		if grow < 0 {
			break
		}
		widths[grow]++
	}
	return widths
}

func columnPriority(column string, index int) int {
	if index == 0 {
		return 1000
	}
	switch strings.ToUpper(strings.TrimSpace(column)) {
	case "NAME":
		return 950
	case "STATE", "STATUS":
		return 900
	case "HEALTH", "SEVERITY":
		return 850
	case "MESSAGE", "DETAIL":
		return 800
	case "PROJECT", "SERVICE", "AGENT":
		return 700
	case "CPU", "MEM", "MEMORY", "DISK", "CTR":
		return 600
	case "AGE", "SEEN", "DURATION":
		return 500
	case "ID", "REVISION", "DIGEST":
		return 300
	default:
		return 400 - index
	}
}

func rightAlignedColumn(column string) bool {
	switch strings.ToUpper(strings.TrimSpace(column)) {
	case "CTR", "CPU", "MEM", "MEMORY", "DISK", "SIZE", "AGE", "SEEN", "DURATION", "SERVICES":
		return true
	default:
		return false
	}
}

func valueStyle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "online", "succeeded", "healthy", "ready", "success":
		return ANSISuccess
	case "failed", "offline", "unhealthy", "dead", "exited", "critical", "unavailable":
		return ANSIError
	case "warning", "starting", "restarting", "degraded", "pending", "building", "releasing":
		return ANSIWarning
	case "running", "active":
		return ANSIInformation
	case "idle", "none", "–":
		return ANSIMuted
	default:
		return ""
	}
}

func hangingWrap(prefix, value string, width int) string {
	prefixWidth := displayWidth(prefix)
	available := max(1, width-prefixWidth)
	wrapped := ansi.Wordwrap(value, available, "")
	wrapped = ansi.Hardwrap(wrapped, available, false)
	lines := strings.Split(wrapped, "\n")
	if len(lines) == 0 {
		return prefix
	}
	indent := strings.Repeat(" ", prefixWidth)
	for index := range lines {
		if index == 0 {
			lines[index] = prefix + lines[index]
		} else {
			lines[index] = indent + lines[index]
		}
	}
	return strings.Join(lines, "\n")
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
