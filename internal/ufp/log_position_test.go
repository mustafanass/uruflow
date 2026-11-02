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
	"testing"
	"time"
)

func TestLogPositionResumesInterleavedStreamsWithoutCollisions(t *testing.T) {
	start := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	late := start.Add(time.Second)
	position := LogPosition{Start: start}
	seen := map[string]bool{}
	for _, line := range []struct {
		stream string
		at     time.Time
	}{{"stdout", late}, {"stderr", start}, {"stdout", late}, {"stderr", start}} {
		position.Advance(line.stream, line.at)
		id := position.String()
		if seen[id] {
			t.Fatal("interleaved stream reused a cursor")
		}
		seen[id] = true
	}
	after, err := ParseLogPosition(position.String())
	if err != nil || !after.Since().Equal(start) {
		t.Fatal(after, err)
	}
	replay := LogPosition{Start: after.Since()}
	checkpoint := after
	emitted := 0
	for _, line := range []struct {
		stream string
		at     time.Time
	}{{"stdout", late}, {"stderr", start}, {"stderr", start}, {"stderr", start}, {"stdout", late}, {"stdout", late}} {
		replay.Advance(line.stream, line.at)
		if after.Includes(line.stream, line.at, replay.Offset(line.stream)) {
			continue
		}
		id := checkpoint.Merge(replay)
		parsed, err := ParseLogPosition(id)
		if err != nil || parsed.Stdout.Offset < 2 {
			t.Fatal("checkpoint moved backwards during replay", parsed, err)
		}
		emitted++
	}
	if emitted != 2 {
		t.Fatal("missed or duplicated lines during replay", emitted)
	}
}

func TestLogPositionAcceptsLegacyCursorsAndRejectsInvalidPositions(t *testing.T) {
	legacy, err := ParseLogPosition("2026-11-01T12:00:00Z/2")
	if err != nil || legacy.Stdout.Offset != 2 || legacy.Stderr.Offset != 2 {
		t.Fatal(legacy, err)
	}
	for _, value := range []string{"v2.broken", "v2.e30", "invalid", "2026-11-01T12:00:00Z/0"} {
		if _, err := ParseLogPosition(value); err == nil {
			t.Fatal("accepted malformed position", value)
		}
	}
}
