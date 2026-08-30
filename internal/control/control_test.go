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

package control

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafanass/uruflow/internal/ops"
)

func TestClientStreamsEveryServerEvent(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "uruflow-control-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "control.sock")
	server, err := Listen(path, func(_ context.Context, args []string, input string, emit ops.Emit) error {
		if len(args) != 1 || args[0] != "status" || input != "payload" {
			t.Fatalf("request args=%v input=%q", args, input)
		}
		if err := emit(ops.Message("info", "first")); err != nil {
			return err
		}
		return emit(ops.Message("success", "second"))
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var messages []string
	err = NewClient(path).Execute(context.Background(), []string{"status"}, "payload", func(event ops.Event) error {
		messages = append(messages, event.Message)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(messages); got != 2 || messages[0] != "first" || messages[1] != "second" {
		t.Fatalf("messages = %#v", messages)
	}
}
