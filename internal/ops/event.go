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

import "time"

const (
	EventMessage = "message"
	EventTable   = "table"
	EventLog     = "log"
	EventResult  = "result"
)

type Event struct {
	Type      string     `json:"type"`
	Sequence  uint64     `json:"sequence,omitempty"`
	Time      time.Time  `json:"time,omitempty"`
	Level     string     `json:"level,omitempty"`
	Operation string     `json:"operation,omitempty"`
	Title     string     `json:"title,omitempty"`
	Message   string     `json:"message,omitempty"`
	Columns   []string   `json:"columns,omitempty"`
	Rows      [][]string `json:"rows,omitempty"`
	Data      any        `json:"data,omitempty"`
}

type Emit func(Event) error

func Message(level, message string) Event {
	return Event{Type: EventMessage, Time: time.Now(), Level: level, Message: message}
}

func Table(title string, columns []string, rows [][]string) Event {
	return Event{Type: EventTable, Time: time.Now(), Title: title, Columns: columns, Rows: rows}
}
