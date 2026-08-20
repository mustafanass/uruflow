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
	"context"
	"fmt"
)

type activeJob struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
}

func (d *Daemon) beginJob(project, jobID string) (*activeJob, bool, error) {
	d.connMu.RLock()
	parent := d.sessionCtx
	d.connMu.RUnlock()
	if parent == nil {
		return nil, false, fmt.Errorf("agent session is not active")
	}

	d.jobMu.Lock()
	defer d.jobMu.Unlock()
	if current := d.jobs[project]; current != nil {
		if current.id == jobID {
			return current, false, nil
		}
		return nil, false, fmt.Errorf("project %s already has job %s running", project, current.id)
	}

	ctx, cancel := context.WithCancel(parent)
	job := &activeJob{id: jobID, ctx: ctx, cancel: cancel}
	d.jobs[project] = job
	return job, true, nil
}

func (d *Daemon) endJob(project string, job *activeJob) {
	d.jobMu.Lock()
	if d.jobs[project] == job {
		delete(d.jobs, project)
	}
	d.jobMu.Unlock()
	job.cancel()
}
