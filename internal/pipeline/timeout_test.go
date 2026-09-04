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
	"strings"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/models"
)

func TestReleaseRequestUsesOnlyTheRemainingProjectTimeout(t *testing.T) {
	project := &models.Project{
		Name: "api-prod", Timeout: 90 * time.Minute,
		Services: []models.Service{{Name: "api", Image: prebuiltImage}},
	}
	release := &models.Release{ID: "r1", StartedAt: time.Now().Add(-30 * time.Minute)}

	request, err := (&Pipeline{}).releaseRequest(release, project)
	if err != nil {
		t.Fatal(err)
	}
	if request.Timeout < 59*time.Minute || request.Timeout > 60*time.Minute {
		t.Fatalf("remaining timeout = %s, want about 60m", request.Timeout)
	}

	release.StartedAt = time.Now().Add(-91 * time.Minute)
	if _, err := (&Pipeline{}).releaseRequest(release, project); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expired release error = %v", err)
	}
}
