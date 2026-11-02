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
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func (c *Collector) getDiskInfo(path string) (uint64, uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free

	return used, total, nil
}

func (c *Collector) getCPUPercent() (float64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}

			var total, idle uint64
			for i := 1; i < len(fields); i++ {
				val, _ := strconv.ParseUint(fields[i], 10, 64)
				total += val
				if i == 4 {
					idle = val
				}
			}

			if c.prevCPUTotal == 0 {
				c.prevCPUIdle = idle
				c.prevCPUTotal = total
				return 0, nil
			}

			if total <= c.prevCPUTotal || idle < c.prevCPUIdle {
				c.prevCPUIdle = idle
				c.prevCPUTotal = total
				return 0, nil
			}

			idleDelta := idle - c.prevCPUIdle
			totalDelta := total - c.prevCPUTotal

			c.prevCPUIdle = idle
			c.prevCPUTotal = total

			if totalDelta == 0 {
				return 0, nil
			}

			cpuPercent := (1.0 - float64(idleDelta)/float64(totalDelta)) * 100

			if cpuPercent < 0 {
				cpuPercent = 0
			}
			if cpuPercent > 100 {
				cpuPercent = 100
			}

			return cpuPercent, nil
		}
	}
	return 0, nil
}

func (c *Collector) getMemoryInfo() (uint64, uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var total, available uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		val, _ := strconv.ParseUint(fields[1], 10, 64)
		val *= 1024

		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			available = val
		}
	}
	return total - available, total, nil
}

func (c *Collector) getLoadAvg() []float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return []float64{0, 0, 0}
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return []float64{0, 0, 0}
	}
	load := make([]float64, 3)
	for i := 0; i < 3; i++ {
		load[i], _ = strconv.ParseFloat(fields[i], 64)
	}
	return load
}

func (c *Collector) getHostUptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	seconds, _ := strconv.ParseFloat(fields[0], 64)
	return int64(seconds)
}
