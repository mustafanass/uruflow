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

package activity

import "testing"

func TestFeedSequencesReplaysAndReportsExactGaps(t *testing.T) {
	feed := New(3)
	for _, message := range []string{"one", "two", "three", "four", "five"} {
		feed.Publish(Entry{Message: message})
	}
	entries, dropped := feed.Read(1)
	if dropped != 1 || len(entries) != 3 || entries[0].Sequence != 3 || entries[2].Message != "five" {
		t.Fatalf("entries=%+v dropped=%d", entries, dropped)
	}
	entries, dropped = feed.Read(4)
	if dropped != 0 || len(entries) != 1 || entries[0].Sequence != 5 {
		t.Fatalf("resume entries=%+v dropped=%d", entries, dropped)
	}
}
