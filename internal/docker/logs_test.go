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

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestResumedDockerLogsKeepTimestampsAndRequestEveryMissedLine(t *testing.T) {
	at := time.Date(2026, 11, 1, 12, 0, 0, 123456789, time.UTC)
	var frames bytes.Buffer
	for index, content := range []string{at.Format(time.RFC3339Nano) + " repeated\n" + at.Format(time.RFC3339Nano) + " repeated\n", at.Add(time.Nanosecond).Format(time.RFC3339Nano) + " diagnostic\n"} {
		header := make([]byte, 8)
		header[0] = byte(index + 1)
		binary.BigEndian.PutUint32(header[4:], uint32(len(content)))
		frames.Write(header)
		frames.WriteString(content)
	}
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		query := r.URL.Query()
		if query.Get("since") != "1793534400.123456789" || query.Get("tail") != "all" || query.Get("timestamps") != "true" {
			t.Fatal(query)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(frames.Bytes()))}, nil
	})}}
	var lines, streams []string
	var timestamps []time.Time
	err := client.StreamLogsSince(context.Background(), "container", 200, at, false, func(stream, line string, timestamp time.Time) {
		streams = append(streams, stream)
		lines = append(lines, line)
		timestamps = append(timestamps, timestamp)
	})
	if err != nil || len(lines) != 3 || lines[0] != "repeated" || lines[1] != "repeated" || streams[2] != "stderr" || !timestamps[2].Equal(at.Add(time.Nanosecond)) {
		t.Fatal(lines, streams, timestamps, err)
	}
}

func TestInitialTailExpandsItsBoundaryBeforeAssigningCursorOffsets(t *testing.T) {
	timestamp := "2026-11-01T12:00:00.123456789Z"
	client := &Client{http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		count := 2
		if r.URL.Query().Get("since") != "" {
			if r.URL.Query().Get("tail") != "all" {
				t.Fatal("initial boundary was truncated again")
			}
			count = 3
		}
		var frames bytes.Buffer
		for i := 0; i < count; i++ {
			content := timestamp + " repeated\n"
			header := make([]byte, 8)
			header[0] = 1
			binary.BigEndian.PutUint32(header[4:], uint32(len(content)))
			frames.Write(header)
			frames.WriteString(content)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(frames.Bytes()))}, nil
	})}}
	start, err := client.LogStart(context.Background(), "container", 2)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	err = client.StreamLogsSince(context.Background(), "container", 2, start, false, func(_, line string, _ time.Time) {
		if line != "repeated" {
			t.Fatal(line)
		}
		count++
	})
	if err != nil || count != 3 {
		t.Fatal("equal timestamps lost their full boundary", count, err)
	}
}
