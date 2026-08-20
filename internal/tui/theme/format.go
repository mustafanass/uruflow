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

package theme

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	ellipsis   = "…"
	escapeByte = 0x1b
)

func Pad(text string, width int) string {
	gap := width - lipgloss.Width(text)
	if gap <= 0 {
		return text
	}
	return text + strings.Repeat(" ", gap)
}

func PadLeft(text string, width int) string {
	gap := width - lipgloss.Width(text)
	if gap <= 0 {
		return text
	}
	return strings.Repeat(" ", gap) + text
}

func Truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return ellipsis
	}
	return ansi.Truncate(text, width, ellipsis)
}

func Upper(text string) string {
	var out strings.Builder
	out.Grow(len(text))

	plain := 0
	for index := 0; index < len(text); {
		if text[index] != escapeByte {
			index++
			continue
		}

		out.WriteString(strings.ToUpper(text[plain:index]))

		end := index + 1
		if end < len(text) && text[end] == '[' {
			end++
			for end < len(text) && (text[end] < 0x40 || text[end] > 0x7e) {
				end++
			}
			if end < len(text) {
				end++
			}
		}

		out.WriteString(text[index:end])
		index = end
		plain = end
	}

	out.WriteString(strings.ToUpper(text[plain:]))
	return out.String()
}

func Sanitize(text string) string {
	return strings.Map(printable, ansi.Strip(text))
}

func printable(value rune) rune {
	switch {
	case value == '\t':
		return ' '
	case value < 0x20 || value == 0x7f:
		return -1
	default:
		return value
	}
}

func Cell(text string, width int) string {
	return Pad(Truncate(text, width), width)
}

func Rows(text string) []string {
	return strings.Split(text, "\n")
}

func Line(width int) string {
	if width <= 0 {
		return ""
	}
	return Rule.Render(strings.Repeat(IconLine, width))
}

func Bytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}

	div, exponent := uint64(unit), 0
	for size := value / unit; size >= unit; size /= unit {
		div *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f%cB", float64(value)/float64(div), "KMGTPE"[exponent])
}

func Duration(milliseconds int64) string {
	if milliseconds <= 0 {
		return "–"
	}

	span := time.Duration(milliseconds) * time.Millisecond
	switch {
	case span < time.Second:
		return fmt.Sprintf("%dms", milliseconds)
	case span < time.Minute:
		return fmt.Sprintf("%.1fs", span.Seconds())
	default:
		return fmt.Sprintf("%dm%02ds", int(span.Minutes()), int(span.Seconds())%60)
	}
}

func Since(moment time.Time) string {
	if moment.IsZero() {
		return "never"
	}

	span := time.Since(moment)
	switch {
	case span < time.Minute:
		return "just now"
	case span < time.Hour:
		return fmt.Sprintf("%dm ago", int(span.Minutes()))
	case span < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(span.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(span.Hours()/24))
	}
}

func Uptime(seconds int64) string {
	if seconds <= 0 {
		return "–"
	}

	span := time.Duration(seconds) * time.Second
	days := int(span.Hours()) / 24
	hours := int(span.Hours()) % 24
	minutes := int(span.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

func ShortSHA(commit string) string {
	const size = 7
	if len(commit) <= size {
		return commit
	}
	return commit[:size]
}

func ImageTag(reference string) string {
	if index := strings.LastIndex(reference, ":"); index >= 0 {
		if tag := reference[index+1:]; !strings.Contains(tag, "/") {
			return tag
		}
	}
	return reference
}

func Frame(index int) string {
	return Lead.Render(Spinner[index%len(Spinner)])
}
