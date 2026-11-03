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

package projects

import "testing"

func TestBuildWatchPathsAreLoadedAndValidated(t *testing.T) {
	base := "builder: build-01\nrunners: [run-01]\nservices:\n  api:\n    git: git@host:app.git\n    branch: main\n    dockerfile: Dockerfile\n"
	environment, err := ParseCreationYAML(base + "    watch: [core, shared, go.mod]\n")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := buildServices(environment.Services)
	if err != nil || len(loaded) != 1 || len(loaded[0].Watch) != 3 {
		t.Fatalf("watch paths lost: %+v %v", loaded, err)
	}
	for _, path := range []string{"../outside", "/absolute", "", "src/**", ".git/HEAD"} {
		t.Run(path, func(t *testing.T) {
			if _, err := ParseCreationYAML(base + "    watch: ['" + path + "']\n"); err == nil {
				t.Fatal("invalid watch path accepted")
			}
		})
	}
}
