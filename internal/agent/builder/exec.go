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

package builder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/mustafanass/uruflow/internal/ufp"
)

type LogFunc = ufp.LogFunc

func stream(ctx context.Context, dir string, log LogFunc, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir

	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return err
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	var wait sync.WaitGroup
	wait.Add(2)
	go scan(&wait, stdout, ufp.StreamStdout, log)
	go scan(&wait, stderr, ufp.StreamStderr, log)
	wait.Wait()

	if err := command.Wait(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func capture(ctx context.Context, dir string, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir

	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func scan(wait *sync.WaitGroup, pipe io.Reader, name string, log LogFunc) {
	defer wait.Done()

	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		if line := scanner.Text(); line != "" && log != nil {
			log(name, line)
		}
	}
}
