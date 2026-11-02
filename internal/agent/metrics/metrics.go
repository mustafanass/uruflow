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
	"fmt"
	"runtime"
	"time"
)

type System struct {
	Available     []string
	Errors        []string
	CPUCores      int
	HostUptime    int64
	CPUPercent    float64
	MemoryPercent float64
	MemoryUsed    uint64
	MemoryTotal   uint64
	DiskPercent   float64
	DiskUsed      uint64
	DiskTotal     uint64
	LoadAvg       []float64
	Uptime        int64
}

type Collector struct {
	prevCPUIdle  uint64
	prevCPUTotal uint64
	bootTime     time.Time
}

func NewCollector() *Collector {
	return &Collector{
		bootTime: time.Now(),
	}
}

func (c *Collector) Collect() (*System, error) {
	m := &System{CPUCores: runtime.NumCPU(), Uptime: int64(time.Since(c.bootTime).Seconds()), HostUptime: c.getHostUptime()}
	primed := c.prevCPUTotal > 0
	if cpu, err := c.getCPUPercent(); err == nil {
		m.CPUPercent = cpu
		if primed {
			m.Available = append(m.Available, "cpu")
		}
	} else {
		m.Errors = append(m.Errors, fmt.Sprintf("CPU: %v", err))
	}
	if used, total, err := c.getMemoryInfo(); err == nil && total > 0 {
		m.MemoryUsed, m.MemoryTotal = used, total
		m.MemoryPercent = float64(used) / float64(total) * 100
		m.Available = append(m.Available, "memory")
	} else {
		m.Errors = append(m.Errors, "Memory collection unavailable")
	}
	if used, total, err := c.getDiskInfo("/"); err == nil && total > 0 {
		m.DiskUsed, m.DiskTotal = used, total
		m.DiskPercent = float64(used) / float64(total) * 100
		m.Available = append(m.Available, "disk")
	} else {
		m.Errors = append(m.Errors, "Root filesystem collection unavailable")
	}
	m.LoadAvg = c.getLoadAvg()
	return m, nil
}
