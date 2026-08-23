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

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"
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
		if err := validateBuildRequest(payload); err != nil {
			return nil, err
		}
		job, start, err := d.beginJob(payload.Project, payload.JobID)
		if err != nil {
			return nil, err
		}
		if start {
			go d.runBuild(payload, job)
		}
		return ufp.Accepted{JobID: payload.JobID}, nil

	case ufp.MethodReleaseRun:
		var payload ufp.ReleaseRequest
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		if !d.cfg.HasRole(ufp.RoleRunner) {
			return nil, fmt.Errorf("agent does not carry the %s role", ufp.RoleRunner)
		}
		if err := validateReleaseRequest(payload); err != nil {
			return nil, err
		}
		job, start, err := d.beginJob(payload.Project, payload.JobID)
		if err != nil {
			return nil, err
		}
		if start {
			go d.runRelease(payload, job)
		}
		return ufp.Accepted{JobID: payload.JobID}, nil

	case ufp.MethodReleaseStop:
		var payload ufp.ProjectRef
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		if !d.cfg.HasRole(ufp.RoleRunner) {
			return nil, fmt.Errorf("agent does not carry the %s role", ufp.RoleRunner)
		}
		if payload.Project == "" {
			return nil, fmt.Errorf("project is required")
		}
		job, start, err := d.beginJob(payload.Project, ufp.MethodReleaseStop)
		if err != nil {
			return nil, err
		}
		if !start {
			return nil, fmt.Errorf("project %s already has a stop operation running", payload.Project)
		}
		defer d.endJob(payload.Project, job)
		ctx, cancel := context.WithTimeout(job.ctx, actionTimeout)
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
		if !d.cfg.HasRole(ufp.RoleRunner) {
			return nil, fmt.Errorf("agent does not carry the %s role", ufp.RoleRunner)
		}
		if payload.Project == "" {
			return nil, fmt.Errorf("project is required")
		}
		job, start, err := d.beginJob(payload.Project, ufp.MethodReleaseRemove)
		if err != nil {
			return nil, err
		}
		if !start {
			return nil, fmt.Errorf("project %s already has a remove operation running", payload.Project)
		}
		defer d.endJob(payload.Project, job)
		ctx, cancel := context.WithTimeout(job.ctx, actionTimeout)
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
		if !d.cfg.HasRole(ufp.RoleRunner) {
			return nil, fmt.Errorf("agent does not carry the %s role", ufp.RoleRunner)
		}
		if payload.ContainerID == "" {
			return nil, fmt.Errorf("container id is required")
		}
		d.followContainer(payload)
		return ufp.Accepted{}, nil

	case ufp.MethodLogsStop:
		var payload ufp.LogsStop
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		if !d.cfg.HasRole(ufp.RoleRunner) {
			return nil, fmt.Errorf("agent does not carry the %s role", ufp.RoleRunner)
		}
		if payload.ContainerID == "" {
			return nil, fmt.Errorf("container id is required")
		}
		d.stopFollow(payload.ContainerID)
		return ufp.Accepted{}, nil
	}

	return nil, fmt.Errorf("unsupported method %q", request.Method)
}

func validateBuildRequest(request ufp.BuildRequest) error {
	if request.JobID == "" || request.Project == "" || request.GitURL == "" || request.Branch == "" {
		return fmt.Errorf("build request is incomplete")
	}
	if request.Commit != "" && !models.ValidGitCommit(request.Commit) {
		return fmt.Errorf("build request has an invalid commit")
	}
	if len(request.Targets) == 0 {
		return fmt.Errorf("build request has no targets")
	}
	seen := make(map[string]bool, len(request.Targets))
	for _, target := range request.Targets {
		if target.Image == "" || target.Dockerfile == "" || target.Context == "" || seen[target.Service] {
			return fmt.Errorf("build target %q is invalid", target.Service)
		}
		seen[target.Service] = true
	}
	return nil
}

func validateReleaseRequest(request ufp.ReleaseRequest) error {
	if request.JobID == "" || request.Project == "" || len(request.Services) == 0 {
		return fmt.Errorf("release request is incomplete")
	}
	seen := make(map[string]bool, len(request.Services))
	for _, service := range request.Services {
		if !models.ValidDigestReference(service.Image) || seen[service.Name] {
			return fmt.Errorf("release service %q is invalid", service.Name)
		}
		for _, port := range service.Ports {
			if port.Host < 0 || port.Host > 65535 || port.Container < 1 || port.Container > 65535 {
				return fmt.Errorf("release service %q has an invalid port", service.Name)
			}
		}
		if service.Healthcheck != nil {
			healthcheck := &models.Healthcheck{
				Type: service.Healthcheck.Type, Scheme: service.Healthcheck.Scheme,
				Path: service.Healthcheck.Path, Port: service.Healthcheck.Port,
				Interval: service.Healthcheck.Interval, Timeout: service.Healthcheck.Timeout,
				Retries: service.Healthcheck.Retries, StableFor: service.Healthcheck.StableFor,
			}
			if err := models.ValidateHealthcheck(healthcheck); err != nil {
				return fmt.Errorf("release service %q: %w", service.Name, err)
			}
		}
		if err := models.ValidateLabels(service.Labels); err != nil {
			return fmt.Errorf("release service %q: %w", service.Name, err)
		}
		seen[service.Name] = true
	}
	return nil
}

func (d *Daemon) HandleEvent(event *ufp.Event) error {
	if event.Topic != ufp.TopicRegistryConfig {
		return fmt.Errorf("unsupported event topic %q", event.Topic)
	}

	var payload ufp.RegistryConfig
	if err := event.Decode(&payload); err != nil {
		return err
	}
	d.connMu.RLock()
	ctx := d.sessionCtx
	d.connMu.RUnlock()
	if ctx == nil {
		return fmt.Errorf("agent session is not active")
	}
	if err := d.applyRegistry(ctx, payload); err != nil {
		return err
	}
	d.send(ufp.TopicRegistryReady, ufp.Accepted{})
	return nil
}

func (d *Daemon) runBuild(request ufp.BuildRequest, job *activeJob) {
	started := time.Now()
	log := d.jobLogger(request.JobID, ufp.StageBuild)

	d.send(ufp.TopicJobStatus, ufp.JobStatus{
		JobID: request.JobID, Stage: ufp.StageBuild, Status: ufp.StatusRunning,
	})
	logger.Info("[AGENT] build %s: %s", request.JobID, request.Project)

	ctx, cancel := context.WithTimeout(job.ctx, buildTimeout)
	defer cancel()

	result, err := d.builder.Build(ctx, request, log)
	d.endJob(request.Project, job)
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

func (d *Daemon) runRelease(request ufp.ReleaseRequest, job *activeJob) {
	started := time.Now()
	log := d.jobLogger(request.JobID, ufp.StageRelease)

	d.send(ufp.TopicJobStatus, ufp.JobStatus{
		JobID: request.JobID, Stage: ufp.StageRelease, Status: ufp.StatusRunning,
	})
	logger.Info("[AGENT] release %s: %s (%d service(s))", request.JobID, request.Project, len(request.Services))

	ctx, cancel := context.WithTimeout(job.ctx, releaseTimeout)
	defer cancel()

	err := d.runner.Release(ctx, request, log)
	d.endJob(request.Project, job)
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
