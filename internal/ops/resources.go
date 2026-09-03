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

	"github.com/mustafanass/uruflow/internal/grammar"
)

func (e *Engine) alerts(args []string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		values, err := e.server.Store().ListActiveAlerts()
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, alert := range values {
			rows = append(rows, []string{alert.ID, string(alert.Severity), alert.AgentName, alert.Type, alert.Message, since(alert.CreatedAt)})
		}
		return emit(Table("alerts", []string{"ID", "SEVERITY", "AGENT", "TYPE", "MESSAGE", "AGE"}, rows))
	}
	if args[0] != "resolve" || len(args) != 2 {
		return grammar.GroupUsageError("alert")
	}
	if err := e.server.Store().ResolveAlert(args[1]); err != nil {
		return err
	}
	return emit(Message("success", "resolved alert "+args[1]))
}

func (e *Engine) registry(ctx context.Context, args []string, emit Emit) error {
	if len(args) == 0 || args[0] == "list" {
		values, err := e.server.Registry().Images(ctx)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(values))
		for _, image := range values {
			rows = append(rows, []string{image.Repository, image.Tag, short(image.Digest, 20), bytes(uint64(max(image.Size, 0))), since(image.CreatedAt)})
		}
		return emit(Table("registry", []string{"REPOSITORY", "TAG", "DIGEST", "SIZE", "AGE"}, rows))
	}
	if args[0] != "remove" || len(args) != 3 {
		return grammar.GroupUsageError("registry")
	}
	if err := e.server.Registry().DeleteTag(ctx, args[1], args[2]); err != nil {
		return err
	}
	return emit(Message("success", "deleted manifest "+args[1]+":"+args[2]))
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
