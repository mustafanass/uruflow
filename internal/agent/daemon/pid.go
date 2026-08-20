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

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/urustack/uruflow/pkg/logger"
)

func (d *Daemon) writePid() error {
	if err := os.MkdirAll(filepath.Dir(d.cfg.PidFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(d.cfg.PidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

func (d *Daemon) removePid() {
	if err := os.Remove(d.cfg.PidFile); err != nil {
		logger.Debug("[AGENT] could not remove the pid file: %v", err)
	}
}

func IsRunning(pidFile string) (bool, int) {
	pid, err := readPid(pidFile)
	if err != nil {
		return false, 0
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	return process.Signal(syscall.Signal(0)) == nil, pid
}

func Stop(pidFile string) error {
	pid, err := readPid(pidFile)
	if err != nil {
		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	logger.Info("[AGENT] signalling process %d to stop", pid)
	return process.Signal(syscall.SIGTERM)
}

func readPid(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, fmt.Errorf("read pid file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}
