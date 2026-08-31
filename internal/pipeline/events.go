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

package pipeline

import (
	"time"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"

	"github.com/mustafanass/uruflow/internal/link"
)

var _ link.Events = (*Pipeline)(nil)

func (p *Pipeline) AgentConnected(agent *models.Agent) {}

func (p *Pipeline) AgentDisconnected(agentID string) {
	releases, err := p.store.ListActiveReleases()
	if err != nil {
		return
	}

	for index := range releases {
		if releases[index].Status.Done() {
			continue
		}
		p.disconnectAgentFromRelease(releases[index].ID, agentID)
	}
}

func (p *Pipeline) disconnectAgentFromRelease(releaseID, agentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	release, err := p.store.GetRelease(releaseID)
	if err != nil || release.Status.Done() {
		return
	}

	if release.Builder == agentID && release.Status == models.StatusBuilding {
		p.failRelease(release, "builder disconnected")
		return
	}

	for _, target := range release.Targets {
		if target.AgentID == agentID && !target.Status.Done() {
			if err := p.finishTarget(release.ID, agentID, target.AgentName, models.StatusFailed, "agent disconnected"); err != nil {
				logger.Error("[PIPELINE] update disconnected target %s/%s: %v", release.ID, agentID, err)
			}
		}
	}
	p.settle(release.ID)
}

func (p *Pipeline) ContainerLog(agentID string, entry ufp.ContainerLog) {}

func (p *Pipeline) JobLog(agentID string, entry ufp.JobLog) {
	release, err := p.store.GetRelease(entry.JobID)
	if err != nil || !p.acceptsEvent(release, agentID, entry.Stage) {
		logger.Warn("[PIPELINE] rejected %s log for %s from %s", entry.Stage, entry.JobID, agentID)
		return
	}
	line := &models.LogLine{
		ReleaseID: entry.JobID,
		Stage:     entry.Stage,
		AgentName: p.agentName(agentID),
		Stream:    entry.Stream,
		Line:      entry.Line,
		Timestamp: time.Unix(entry.Timestamp, 0),
	}
	if err := p.store.AppendLog(line); err != nil {
		logger.Error("[PIPELINE] append log for %s: %v", entry.JobID, err)
		return
	}
	p.publishLog(line)
}

func (p *Pipeline) JobStatus(agentID string, status ufp.JobStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()

	release, err := p.store.GetRelease(status.JobID)
	if err != nil {
		return
	}
	if !p.acceptsEvent(release, agentID, status.Stage) {
		logger.Warn("[PIPELINE] rejected %s status for %s from %s", status.Stage, status.JobID, agentID)
		return
	}

	switch status.Stage {
	case ufp.StageBuild:
		p.onBuildStatus(release, status)
	case ufp.StageRelease:
		p.onReleaseStatus(release, agentID, status)
	}
}

func (p *Pipeline) acceptsEvent(release *models.Release, agentID, stage string) bool {
	switch stage {
	case ufp.StageBuild:
		return release.Status == models.StatusBuilding && release.Builder == agentID
	case ufp.StageRelease:
		if release.Status != models.StatusReleasing {
			return false
		}
		for _, target := range release.Targets {
			if target.AgentID == agentID && !target.Status.Done() {
				return true
			}
		}
	}
	return false
}

func (p *Pipeline) onBuildStatus(release *models.Release, status ufp.JobStatus) {
	switch status.Status {
	case ufp.StatusFailed:
		p.failRelease(release, status.Message)

	case ufp.StatusSuccess:
		if err := p.validateBuildResult(release, status); err != nil {
			p.failRelease(release, err.Error())
			return
		}
		release.Image = status.Image
		release.Images = status.Images
		release.Commits = status.Commits
		release.Digest = status.Digest
		if status.Commit != "" {
			release.Commit = status.Commit
		}
		if release.Spec.EffectiveWorkflow() == models.WorkflowBuildOnly {
			p.completeRelease(release, models.StatusSucceeded, "")
			logger.Info("[PIPELINE] release %s: build completed without deployment", release.ID)
			return
		}
		release.Status = models.StatusReleasing
		if err := p.store.UpdateRelease(release); err != nil {
			logger.Error("[PIPELINE] update release %s: %v", release.ID, err)
			return
		}
		p.publishRelease(release)

		project := &release.Spec
		if project.Name == "" {
			var err error
			project, err = p.store.GetProject(release.Project)
			if err != nil {
				p.failRelease(release, "release has no project snapshot")
				return
			}
		}

		runners, err := p.resolveRunners(project)
		if err != nil {
			p.failRelease(release, err.Error())
			return
		}

		logger.Info("[PIPELINE] release %s: built %s, rolling out to %d runner(s)",
			release.ID, release.Image, len(runners))
		go p.rollout(release, project, runners)
	}
}

func (p *Pipeline) onReleaseStatus(release *models.Release, agentID string, status ufp.JobStatus) {
	if status.Status == ufp.StatusRunning {
		return
	}

	var outcome models.Status
	switch status.Status {
	case ufp.StatusSuccess:
		outcome = models.StatusSucceeded
	case ufp.StatusFailed:
		outcome = models.StatusFailed
	default:
		logger.Warn("[PIPELINE] rejected unknown release status %q for %s", status.Status, status.JobID)
		return
	}

	if err := p.finishTarget(release.ID, agentID, p.agentName(agentID), outcome, status.Message); err != nil {
		logger.Error("[PIPELINE] update target %s/%s: %v", release.ID, agentID, err)
		return
	}
	p.settle(release.ID)
}

func (p *Pipeline) finishTarget(releaseID, agentID, agentName string, status models.Status, message string) error {
	ended := time.Now()
	return p.store.SaveReleaseTarget(&models.ReleaseTarget{
		ReleaseID: releaseID,
		AgentID:   agentID,
		AgentName: agentName,
		Status:    status,
		Message:   message,
		EndedAt:   &ended,
	})
}

func (p *Pipeline) settle(releaseID string) {
	release, err := p.store.GetRelease(releaseID)
	if err != nil || release.Status.Done() {
		return
	}

	succeeded, failed, skipped := 0, 0, 0
	for _, target := range release.Targets {
		if !target.Status.Done() {
			return
		}
		switch target.Status {
		case models.StatusSucceeded:
			succeeded++
		case models.StatusFailed:
			failed++
		case models.StatusSkipped:
			skipped++
		}
	}

	if failed > 0 || skipped > 0 || succeeded == 0 {
		message := "no runner accepted the release"
		if failed > 0 {
			message = "one or more runners failed"
		} else if skipped > 0 {
			message = "one or more runners were unavailable"
		}
		p.failRelease(release, message)
		return
	}

	p.completeRelease(release, models.StatusSucceeded, "")
	logger.Info("[PIPELINE] release %s: %s live on %d runner(s)", release.ID, release.Image, succeeded)
}

func (p *Pipeline) failRelease(release *models.Release, message string) {
	current, err := p.store.GetRelease(release.ID)
	if err == nil {
		for _, target := range current.Targets {
			if target.Status.Done() {
				continue
			}
			if err := p.finishTarget(release.ID, target.AgentID, target.AgentName,
				models.StatusSkipped, message); err != nil {
				logger.Error("[PIPELINE] close target %s/%s: %v", release.ID, target.AgentID, err)
			}
		}
	}
	p.completeRelease(release, models.StatusFailed, message)
	logger.Error("[PIPELINE] release %s failed: %s", release.ID, message)
}

func (p *Pipeline) completeRelease(release *models.Release, status models.Status, message string) {
	ended := time.Now()
	release.Status = status
	release.Message = message
	release.EndedAt = &ended
	release.Duration = ended.Sub(release.StartedAt).Milliseconds()
	if release.Duration < 1 {
		release.Duration = 1
	}
	if err := p.store.UpdateRelease(release); err != nil {
		logger.Error("[PIPELINE] complete release %s: %v", release.ID, err)
		return
	}
	p.publishRelease(release)
}

func (p *Pipeline) agentName(agentID string) string {
	agent, err := p.store.GetAgent(agentID)
	if err != nil {
		return agentID
	}
	return agent.Name
}
