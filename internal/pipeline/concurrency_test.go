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

package pipeline

import (
	"errors"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
)

func TestConcurrentReleasesAreRejected(t *testing.T) {
	harness := newHarness(t)
	first, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	<-harness.agent.builds

	if _, err := harness.pipeline.Trigger("api", "", models.TriggerManual); !errors.Is(err, ErrReleaseInFlight) {
		t.Fatalf("second trigger error = %v, want ErrReleaseInFlight", err)
	}
	if _, err := harness.pipeline.Rollback("api", ""); !errors.Is(err, ErrReleaseInFlight) {
		t.Fatalf("rollback during a release = %v, want ErrReleaseInFlight", err)
	}

	<-harness.agent.releases
	harness.await(t, first.ID, models.StatusSucceeded)
	second, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger once the first settled: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("the second release reused the first release id")
	}
}

func TestConcurrentTriggersOnlyOneWins(t *testing.T) {
	harness := newHarness(t)
	const attempts = 8
	results := make(chan error, attempts)
	for range attempts {
		go func() {
			_, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
			results <- err
		}()
	}

	accepted := 0
	for range attempts {
		if err := <-results; err == nil {
			accepted++
		} else if !errors.Is(err, ErrReleaseInFlight) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if accepted != 1 {
		t.Fatalf("%d concurrent triggers were accepted, want exactly 1", accepted)
	}
}

func TestTriggerRejectsUnknownProject(t *testing.T) {
	harness := newHarness(t)
	if _, err := harness.pipeline.Trigger("ghost", "", models.TriggerManual); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("error = %v, want ErrProjectNotFound", err)
	}
}

func TestStopReleasesFleetMutexDuringAgentRoundTrip(t *testing.T) {
	harness := newHarness(t)
	harness.agent.holdOperation = make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- harness.pipeline.Stop("api") }()

	select {
	case method := <-harness.agent.operations:
		if method != ufp.MethodReleaseStop {
			t.Fatalf("operation = %q", method)
		}
	case <-time.After(settleWindow):
		t.Fatal("stop did not reach the agent")
	}

	available := make(chan struct{})
	go func() {
		harness.pipeline.mu.Lock()
		harness.pipeline.mu.Unlock()
		close(available)
	}()
	select {
	case <-available:
	case <-time.After(time.Second):
		t.Fatal("stop held the fleet mutex during the agent round-trip")
	}

	if _, err := harness.pipeline.Trigger("api", "", models.TriggerManual); !errors.Is(err, ErrProjectBusy) {
		t.Fatalf("trigger during stop = %v, want ErrProjectBusy", err)
	}
	close(harness.agent.holdOperation)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
