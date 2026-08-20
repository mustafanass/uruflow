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
	"strings"
)

const (
	streamHeaderSize = 8
	maxLogLine       = 1 << 20
	stderrDescriptor = 2
)

func (c *Client) StreamLogs(ctx context.Context, id string, tail int, follow bool, onLine func(stream, line string)) error {
	path := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&timestamps=false&tail=%d&follow=%t",
		id, tail, follow)

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

		for _, line := range strings.Split(strings.TrimRight(string(content), "\n"), "\n") {
			if line != "" && onLine != nil {
				onLine(stream, line)
			}
		}
	}
}
