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

package daemon

import (
	"context"
	"testing"
)

func TestProjectJobsAreIdempotentAndExclusive(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	daemon := &Daemon{sessionCtx: parent, jobs: make(map[string]*activeJob)}

	first, start, err := daemon.beginJob("api", "release-1")
	if err != nil || !start {
		t.Fatalf("first job: start=%v err=%v", start, err)
	}
	same, start, err := daemon.beginJob("api", "release-1")
	if err != nil || start || same != first {
		t.Fatalf("duplicate job: same=%v start=%v err=%v", same == first, start, err)
	}
	if _, _, err := daemon.beginJob("api", "release-2"); err == nil {
		t.Fatal("a second job for the same project was accepted")
	}

	cancel()
	select {
	case <-first.ctx.Done():
	default:
		t.Fatal("job survived its agent session")
	}

	daemon.endJob("api", first)
	if _, exists := daemon.jobs["api"]; exists {
		t.Fatal("completed job remained active")
	}
}
