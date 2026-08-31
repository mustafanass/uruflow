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
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestContainerBridgeAppliesBackpressureInsteadOfDroppingLogs(t *testing.T) {
	done := make(chan struct{})
	bridge := &containerBridge{agentID: "a1", container: "c1", entries: make(chan ufp.ContainerLog, 1), done: done}
	first := ufp.ContainerLog{ContainerID: "c1", Line: "first"}
	second := ufp.ContainerLog{ContainerID: "c1", Line: "second"}
	bridge.ContainerLog("a1", first)
	started := make(chan struct{})
	completed := make(chan struct{})
	go func() {
		close(started)
		bridge.ContainerLog("a1", second)
		close(completed)
	}()
	<-started
	select {
	case <-completed:
		t.Fatal("a full bridge silently accepted or dropped the second line")
	case <-time.After(20 * time.Millisecond):
	}
	if got := <-bridge.entries; got.Line != "first" {
		t.Fatalf("first line = %q", got.Line)
	}
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("bridge did not resume after the consumer advanced")
	}
	if got := <-bridge.entries; got.Line != "second" {
		t.Fatalf("second line = %q", got.Line)
	}
	close(done)
}
