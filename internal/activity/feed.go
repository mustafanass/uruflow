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

package activity

import (
	"sync"
	"time"
)

const DefaultCapacity = 8192

type Kind string

const (
	KindMessage Kind = "message"
	KindLog     Kind = "log"
)

type Entry struct {
	Sequence  uint64    `json:"sequence"`
	Time      time.Time `json:"time"`
	Kind      Kind      `json:"kind"`
	Level     string    `json:"level"`
	Operation string    `json:"operation"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
}

type Feed struct {
	mu      sync.Mutex
	entries []Entry
	start   int
	count   int
	next    uint64
}

func New(capacity int) *Feed {
	if capacity < 1 {
		capacity = DefaultCapacity
	}
	return &Feed{entries: make([]Entry, capacity)}
}

func (f *Feed) Publish(entry Entry) Entry {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.next++
	entry.Sequence = f.next
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	if f.count < len(f.entries) {
		index := (f.start + f.count) % len(f.entries)
		f.entries[index] = entry
		f.count++
	} else {
		f.entries[f.start] = entry
		f.start = (f.start + 1) % len(f.entries)
	}
	return entry
}

func (f *Feed) Read(after uint64) ([]Entry, uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.readLocked(after)
}

func (f *Feed) readLocked(after uint64) ([]Entry, uint64) {
	if f.count == 0 {
		return nil, 0
	}
	oldest := f.entries[f.start].Sequence
	dropped := uint64(0)
	if after < oldest-1 {
		dropped = oldest - after - 1
	}
	result := make([]Entry, 0, f.count)
	for offset := 0; offset < f.count; offset++ {
		entry := f.entries[(f.start+offset)%len(f.entries)]
		if entry.Sequence > after {
			result = append(result, entry)
		}
	}
	return result, dropped
}
