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

package docker

import "testing"

func TestStateFailedDetectsTerminalContainers(t *testing.T) {
	cases := []struct {
		name  string
		state State
		fails bool
	}{
		{"crashed on boot", State{Status: StateExited, ExitCode: 1, Health: HealthNone}, true},
		{"clean exit still counts as gone", State{Status: StateExited, Health: HealthNone}, true},
		{"dead", State{Status: StateDead, Health: HealthNone}, true},
		{"healthcheck failed", State{Status: StateRunning, Health: HealthUnhealthy}, true},
		{"still starting", State{Status: StateRunning, Health: HealthStarting}, false},
		{"restarting", State{Status: StateRestarting, Health: HealthNone}, false},
		{"healthy", State{Status: StateRunning, Health: HealthHealthy}, false},
		{"running without a healthcheck", State{Status: StateRunning, Health: HealthNone}, false},
	}

	for _, test := range cases {
		if failed := test.state.Failed() != nil; failed != test.fails {
			t.Errorf("%s: Failed() = %v, want %v", test.name, failed, test.fails)
		}
	}
}

func TestStateReadyOnlyWhenRunning(t *testing.T) {
	cases := []struct {
		name  string
		state State
		ready bool
	}{
		{"healthy", State{Status: StateRunning, Health: HealthHealthy}, true},
		{"running without a healthcheck", State{Status: StateRunning, Health: HealthNone}, true},
		{"still starting", State{Status: StateRunning, Health: HealthStarting}, false},
		{"unhealthy", State{Status: StateRunning, Health: HealthUnhealthy}, false},
		{"restarting", State{Status: StateRestarting, Health: HealthNone}, false},
		{"exited", State{Status: StateExited, Health: HealthNone}, false},
	}

	for _, test := range cases {
		if ready := test.state.Ready(); ready != test.ready {
			t.Errorf("%s: Ready() = %v, want %v", test.name, ready, test.ready)
		}
	}
}
