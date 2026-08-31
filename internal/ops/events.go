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
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/mustafanass/uruflow/internal/activity"
	"github.com/mustafanass/uruflow/internal/grammar"
)

func (e *Engine) events(ctx context.Context, args []string, emit Emit) error {
	feed := e.server.Activity()
	if feed == nil {
		return errors.New("activity stream is unavailable")
	}
	after := feed.Latest()
	if len(args) > 0 {
		if len(args) != 2 || args[0] != "--after" {
			return grammar.UsageError("events")
		}
		parsed, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return errors.New("event sequence must be a non-negative integer")
		}
		after = parsed
		if latest := feed.Latest(); after > latest {
			return fmt.Errorf("event sequence %d is ahead of the latest sequence %d", after, latest)
		}
	}
	if err := emit(Event{Type: EventMessage, Level: "success",
		Message: fmt.Sprintf("following server activity after sequence %d", after)}); err != nil {
		return err
	}
	for {
		entries, dropped, err := feed.Wait(ctx, after)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		if dropped > 0 {
			resume := after + dropped + 1
			if len(entries) > 0 {
				resume = entries[0].Sequence
			}
			if err := emit(Event{Type: EventMessage, Level: "warning", Sequence: resume - 1,
				Message: fmt.Sprintf("%d activity entries dropped · resumed at sequence %d", dropped, resume)}); err != nil {
				return err
			}
		}
		for _, entry := range entries {
			if err := emit(activityEvent(entry)); err != nil {
				return err
			}
			after = entry.Sequence
		}
	}
}

func activityEvent(entry activity.Entry) Event {
	eventType := EventMessage
	if entry.Kind == activity.KindLog {
		eventType = EventLog
	}
	return Event{Type: eventType, Sequence: entry.Sequence, Time: entry.Time, Level: entry.Level,
		Operation: entry.Operation, Title: entry.Source, Message: entry.Message}
}
