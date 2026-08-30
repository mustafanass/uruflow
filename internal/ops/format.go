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

package ops

import (
	"fmt"
	"strings"
	"time"
)

func bytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exponent := uint64(unit), 0
	for value/div >= unit && exponent < 5 {
		div *= unit
		exponent++
	}
	scaled := float64(value) / float64(div)
	unitName := string("KMGTPE"[exponent]) + "B"
	if scaled >= 100 || scaled == float64(int64(scaled)) {
		return fmt.Sprintf("%.0f %s", scaled, unitName)
	}
	return fmt.Sprintf("%.1f %s", scaled, unitName)
}

func since(moment time.Time) string {
	if moment.IsZero() {
		return "never"
	}
	span := time.Since(moment)
	switch {
	case span < time.Minute:
		return "now"
	case span < time.Hour:
		return fmt.Sprintf("%dm", int(span.Minutes()))
	case span < 24*time.Hour:
		return fmt.Sprintf("%dh", int(span.Hours()))
	default:
		return fmt.Sprintf("%dd", int(span.Hours()/24))
	}
}

func duration(milliseconds int64) string {
	if milliseconds <= 0 {
		return "–"
	}
	span := time.Duration(milliseconds) * time.Millisecond
	if span < time.Minute {
		return fmt.Sprintf("%.1fs", span.Seconds())
	}
	return fmt.Sprintf("%dm %02ds", int(span.Minutes()), int(span.Seconds())%60)
}

func short(value string, size int) string {
	if len(value) <= size {
		return value
	}
	return value[:size]
}

func list(values []string) string {
	if len(values) == 0 {
		return "–"
	}
	return strings.Join(values, ",")
}
