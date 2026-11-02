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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type LogPosition struct {
	Start  time.Time `json:"start"`
	Stdout LogCursor `json:"out"`
	Stderr LogCursor `json:"err"`
}

func ParseLogPosition(value string) (LogPosition, error) {
	if value == "" {
		return LogPosition{}, nil
	}
	if !strings.HasPrefix(value, "v2.") {
		cursor, err := ParseLogCursor(value)
		return LogPosition{Start: cursor.Time, Stdout: cursor, Stderr: cursor}, err
	}
	if len(value) > 1024 {
		return LogPosition{}, fmt.Errorf("invalid log position")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "v2."))
	if err != nil {
		return LogPosition{}, fmt.Errorf("invalid log position")
	}
	var position LogPosition
	if json.Unmarshal(raw, &position) != nil || position.Start.Unix() <= 0 {
		return LogPosition{}, fmt.Errorf("invalid log position")
	}
	for _, cursor := range []LogCursor{position.Stdout, position.Stderr} {
		if cursor.Time.IsZero() && cursor.Offset == 0 {
			continue
		}
		if cursor.Time.Unix() <= 0 || cursor.Offset == 0 || cursor.Time.Before(position.Start) {
			return LogPosition{}, fmt.Errorf("invalid log position")
		}
	}
	return position, nil
}

func (p LogPosition) Since() time.Time {
	since := p.Stdout.Time
	if since.IsZero() {
		since = p.Start
	}
	other := p.Stderr.Time
	if other.IsZero() {
		other = p.Start
	}
	if other.Before(since) {
		since = other
	}
	return since
}

func (p *LogPosition) Advance(stream string, at time.Time) {
	cursor := &p.Stdout
	if stream == "stderr" {
		cursor = &p.Stderr
	}
	cursor.Advance(at)
}

func (p LogPosition) Includes(stream string, at time.Time, offset uint32) bool {
	cursor := p.Stdout
	if stream == "stderr" {
		cursor = p.Stderr
	}
	return !cursor.Time.IsZero() && (at.Before(cursor.Time) || at.Equal(cursor.Time) && offset <= cursor.Offset)
}

func (p LogPosition) Offset(stream string) uint32 {
	if stream == "stderr" {
		return p.Stderr.Offset
	}
	return p.Stdout.Offset
}

func (p LogPosition) String() string {
	data, _ := json.Marshal(p)
	return "v2." + base64.RawURLEncoding.EncodeToString(data)
}

func (p *LogPosition) Merge(replay LogPosition) string {
	for _, pair := range []struct {
		saved *LogCursor
		next  LogCursor
	}{{&p.Stdout, replay.Stdout}, {&p.Stderr, replay.Stderr}} {
		if pair.next.Time.After(pair.saved.Time) || pair.next.Time.Equal(pair.saved.Time) && pair.next.Offset > pair.saved.Offset {
			*pair.saved = pair.next
		}
	}
	return p.String()
}
