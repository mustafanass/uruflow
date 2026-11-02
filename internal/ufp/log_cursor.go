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
	"fmt"
	"strconv"
	"strings"
	"time"
)

type LogCursor struct {
	Time   time.Time
	Offset uint32
}

func ParseLogCursor(value string) (LogCursor, error) {
	if value == "" {
		return LogCursor{}, nil
	}
	timestamp, ordinal, ok := strings.Cut(value, "/")
	if !ok || len(value) > 96 {
		return LogCursor{}, fmt.Errorf("invalid log cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil || at.Unix() <= 0 {
		return LogCursor{}, fmt.Errorf("invalid log cursor timestamp")
	}
	offset, err := strconv.ParseUint(ordinal, 10, 32)
	if err != nil || offset == 0 {
		return LogCursor{}, fmt.Errorf("invalid log cursor offset")
	}
	return LogCursor{Time: at.UTC(), Offset: uint32(offset)}, nil
}

func (c LogCursor) String() string {
	if c.Time.IsZero() {
		return ""
	}
	return c.Time.UTC().Format(time.RFC3339Nano) + "/" + strconv.FormatUint(uint64(c.Offset), 10)
}

func (c *LogCursor) Advance(at time.Time) string {
	if !c.Time.Equal(at) {
		c.Time = at
		c.Offset = 0
	}
	c.Offset++
	return c.String()
}
