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

package metrics

import (
	"errors"
	"syscall"
)

func (c *Collector) getDiskInfo(path string) (uint64, uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}

	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	free := uint64(stat.Bfree) * uint64(stat.Bsize)
	used := total - free

	return used, total, nil
}

func (c *Collector) getCPUPercent() (float64, error) {
	return 0, errors.New("unsupported platform")
}

func (c *Collector) getMemoryInfo() (uint64, uint64, error) {
	return 0, 0, errors.New("unsupported platform")
}

func (c *Collector) getLoadAvg() []float64 {
	return nil
}

func (c *Collector) getHostUptime() int64 { return 0 }
