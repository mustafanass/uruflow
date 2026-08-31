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
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/activity"
	"github.com/mustafanass/uruflow/internal/models"
)

func TestPipelinePublishesReleaseStateAndAcceptedLogs(t *testing.T) {
	feed := activity.New(8)
	pipeline := &Pipeline{activity: feed}
	pipeline.publishRelease(&models.Release{ID: "r1", Project: "api-prod", Status: models.StatusBuilding})
	pipeline.publishLog(&models.LogLine{ReleaseID: "r1", AgentName: "builder-01", Stream: "stderr",
		Line: "compile warning", Timestamp: time.Now()})
	entries, dropped := feed.Read(0)
	if dropped != 0 || len(entries) != 2 {
		t.Fatalf("entries=%+v dropped=%d", entries, dropped)
	}
	if entries[0].Sequence != 1 || entries[0].Message != "api-prod · building" || entries[0].Operation != "r1" {
		t.Fatalf("release activity=%+v", entries[0])
	}
	if entries[1].Kind != activity.KindLog || entries[1].Level != "warning" || entries[1].Source != "builder-01" {
		t.Fatalf("log activity=%+v", entries[1])
	}
}
