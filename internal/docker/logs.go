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
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	streamHeaderSize = 8
	maxLogLine       = 1 << 20
	stderrDescriptor = 2
)

func (c *Client) StreamLogs(ctx context.Context, id string, tail int, follow bool, onLine func(stream, line string)) error {
	query := url.Values{"stdout": {"true"}, "stderr": {"true"}, "tail": {strconv.Itoa(tail)}, "follow": {strconv.FormatBool(follow)}}
	return c.streamLogFrames(ctx, id, query, func(stream, content string) error {
		for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
			if line != "" && onLine != nil {
				onLine(stream, line)
			}
		}
		return nil
	})
}

func (c *Client) StreamLogsSince(ctx context.Context, id string, tail int, since time.Time, follow bool, onLine func(string, string, time.Time)) error {
	query := url.Values{"stdout": {"true"}, "stderr": {"true"}, "timestamps": {"true"}, "tail": {strconv.Itoa(tail)}, "follow": {strconv.FormatBool(follow)}}
	if !since.IsZero() {
		query.Set("since", fmt.Sprintf("%d.%09d", since.Unix(), since.Nanosecond()))
		query.Set("tail", "all")
	}
	return c.streamLogFrames(ctx, id, query, func(stream, content string) error {
		var at time.Time
		for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
			timestamp, text, found := strings.Cut(line, " ")
			parsed, err := time.Parse(time.RFC3339Nano, timestamp)
			if found && err == nil {
				at = parsed
				line = text
			} else if at.IsZero() {
				return fmt.Errorf("docker: log entry is missing its timestamp")
			}
			if onLine != nil {
				onLine(stream, line, at)
			}
		}
		return nil
	})
}

func (c *Client) LogStart(ctx context.Context, id string, tail int) (time.Time, error) {
	var start time.Time
	err := c.StreamLogsSince(ctx, id, tail, time.Time{}, false, func(_, _ string, at time.Time) {
		if start.IsZero() || at.Before(start) {
			start = at
		}
	})
	if start.IsZero() {
		start = time.Unix(1, 0)
	}
	return start, err
}

func (c *Client) streamLogFrames(ctx context.Context, id string, query url.Values, emit func(string, string) error) error {
	path := "/containers/" + url.PathEscape(id) + "/logs?" + query.Encode()

	response, err := c.do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	reader := bufio.NewReader(response.Body)
	header := make([]byte, streamHeaderSize)

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		if _, err := io.ReadFull(reader, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}

		size := binary.BigEndian.Uint32(header[4:])
		if size == 0 {
			continue
		}
		if size > maxLogLine {
			return fmt.Errorf("docker: log frame too large (%d bytes)", size)
		}

		content := make([]byte, size)
		if _, err := io.ReadFull(reader, content); err != nil {
			return err
		}

		stream := "stdout"
		if header[0] == stderrDescriptor {
			stream = "stderr"
		}

		if err := emit(stream, string(content)); err != nil {
			return err
		}
	}
}
