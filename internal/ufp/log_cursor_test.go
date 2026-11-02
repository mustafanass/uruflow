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

package ufp

import (
	"strings"
	"testing"
	"time"
)

func TestLogCursorKeepsRepeatedLinesDistinctAndResumesAtTheBoundary(t *testing.T) {
	at := time.Date(2026, 11, 1, 12, 0, 0, 123456789, time.UTC)
	cursor := LogCursor{}
	first, second := cursor.Advance(at), cursor.Advance(at)
	if first == second {
		t.Fatal("same timestamp collapsed separate log entries")
	}
	resumed, err := ParseLogCursor(second)
	if err != nil || !resumed.Time.Equal(at) || resumed.Offset != 2 {
		t.Fatal(resumed, err)
	}
	if resumed.String() != second {
		t.Fatal("cursor did not round trip")
	}
	next := cursor.Advance(at.Add(time.Nanosecond))
	parsed, err := ParseLogCursor(next)
	if err != nil || parsed.Offset != 1 {
		t.Fatal(parsed, err)
	}
}

func TestLogCursorRejectsMalformedAndInjectedValues(t *testing.T) {
	for _, value := range []string{"broken", "2026-11-01T12:00:00Z/0", "2026-11-01T12:00:00Z/-1", "2026-11-01T12:00:00Z/4294967296", "2026-11-01T12:00:00Z/1\nevent: complete", strings.Repeat("x", 128)} {
		if _, err := ParseLogCursor(value); err == nil {
			t.Fatal("accepted invalid cursor", value)
		}
	}
}
