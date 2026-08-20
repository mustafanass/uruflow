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
	"time"

	"github.com/urustack/uruflow/internal/ufp"
	"github.com/urustack/uruflow/pkg/logger"
)

const (
	buildTimeout   = 30 * time.Minute
	releaseTimeout = 10 * time.Minute
	actionTimeout  = 2 * time.Minute
)

func (d *Daemon) HandleRequest(request *ufp.Request) (any, error) {
	switch request.Method {
	case ufp.MethodBuildRun:
		var payload ufp.BuildRequest
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		if d.builder == nil {
			return nil, fmt.Errorf("agent does not carry the %s role", ufp.RoleBuilder)
		}
		go d.runBuild(payload)
		return ufp.Accepted{JobID: payload.JobID}, nil

	case ufp.MethodReleaseRun:
		var payload ufp.ReleaseRequest
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		if !d.cfg.HasRole(ufp.RoleRunner) {
			return nil, fmt.Errorf("agent does not carry the %s role", ufp.RoleRunner)
		}
		go d.runRelease(payload)
		return ufp.Accepted{JobID: payload.JobID}, nil

	case ufp.MethodReleaseStop:
		var payload ufp.ProjectRef
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		if err := d.runner.Stop(ctx, payload.Project); err != nil {
			return nil, err
		}
		logger.Info("[AGENT] stopped %s", payload.Project)
		return ufp.Accepted{}, nil

	case ufp.MethodReleaseRemove:
		var payload ufp.ProjectRef
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		if err := d.runner.Remove(ctx, payload.Project); err != nil {
			return nil, err
		}
		logger.Info("[AGENT] removed %s", payload.Project)
		return ufp.Accepted{}, nil

	case ufp.MethodLogsFollow:
		var payload ufp.LogsFollow
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		d.followContainer(payload)
		return ufp.Accepted{}, nil

	case ufp.MethodLogsStop:
		var payload ufp.LogsStop
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		d.stopFollow(payload.ContainerID)
		return ufp.Accepted{}, nil
	}

	return nil, fmt.Errorf("unsupported method %q", request.Method)
}

func (d *Daemon) HandleEvent(event *ufp.Event) error {
	if event.Topic != ufp.TopicRegistryConfig {
		return nil
	}

	var payload ufp.RegistryConfig
	if err := event.Decode(&payload); err != nil {
		return nil
	}
	d.applyRegistry(payload)
	return nil
}

func (d *Daemon) runBuild(request ufp.BuildRequest) {
	started := time.Now()
	log := d.jobLogger(request.JobID, ufp.StageBuild)

	d.send(ufp.TopicJobStatus, ufp.JobStatus{
		JobID: request.JobID, Stage: ufp.StageBuild, Status: ufp.StatusRunning,
	})
	logger.Info("[AGENT] build %s: %s", request.JobID, request.Project)

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	result, err := d.builder.Build(ctx, request, log)
	if err != nil {
		log(ufp.StreamStderr, err.Error())
		d.finishJob(request.JobID, ufp.StageBuild, err, started, ufp.JobStatus{})
		return
	}

	logger.Info("[AGENT] build %s: pushed %d image(s)", request.JobID, len(result.Images))
	d.finishJob(request.JobID, ufp.StageBuild, nil, started, ufp.JobStatus{
		Image:  result.Image,
		Images: result.Images,
		Commit: result.Commit,
		Digest: result.Digest,
	})
}

func (d *Daemon) runRelease(request ufp.ReleaseRequest) {
	started := time.Now()
	log := d.jobLogger(request.JobID, ufp.StageRelease)

	d.send(ufp.TopicJobStatus, ufp.JobStatus{
		JobID: request.JobID, Stage: ufp.StageRelease, Status: ufp.StatusRunning,
	})
	logger.Info("[AGENT] release %s: %s (%d service(s))", request.JobID, request.Project, len(request.Services))

	ctx, cancel := context.WithTimeout(context.Background(), releaseTimeout)
	defer cancel()

	err := d.runner.Release(ctx, request, log)
	if err != nil {
		log(ufp.StreamStderr, err.Error())
	}
	d.finishJob(request.JobID, ufp.StageRelease, err, started, ufp.JobStatus{})
}

func (d *Daemon) finishJob(jobID, stage string, failure error, started time.Time, status ufp.JobStatus) {
	status.JobID = jobID
	status.Stage = stage
	status.Status = ufp.StatusSuccess
	status.Duration = time.Since(started).Milliseconds()

	if failure != nil {
		status.Status = ufp.StatusFailed
		status.Message = failure.Error()
		logger.Error("[AGENT] %s %s failed: %v", stage, jobID, failure)
	}

	d.send(ufp.TopicJobStatus, status)
}

func (d *Daemon) jobLogger(jobID, stage string) ufp.LogFunc {
	return func(stream, line string) {
		d.send(ufp.TopicJobLog, ufp.JobLog{
			JobID:     jobID,
			Stage:     stage,
			Stream:    stream,
			Line:      line,
			Timestamp: time.Now().Unix(),
		})
	}
}
