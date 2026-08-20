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
	releases, err := p.store.ListReleases(activeReleaseWindow)
	if err != nil {
		return
	}

	for index := range releases {
		if releases[index].Status.Done() {
			continue
		}

		release, err := p.store.GetRelease(releases[index].ID)
		if err != nil {
			continue
		}

		if release.Builder == agentID && release.Status == models.StatusBuilding {
			p.failRelease(release, "builder disconnected")
			continue
		}

		for _, target := range release.Targets {
			if target.AgentID == agentID && !target.Status.Done() {
				p.finishTarget(release.ID, agentID, target.AgentName, models.StatusFailed, "agent disconnected")
			}
		}
		p.settle(release.ID)
	}
}

func (p *Pipeline) ContainerLog(agentID string, entry ufp.ContainerLog) {}

func (p *Pipeline) JobLog(agentID string, entry ufp.JobLog) {
	p.store.AppendLog(&models.LogLine{
		ReleaseID: entry.JobID,
		Stage:     entry.Stage,
		AgentName: p.agentName(agentID),
		Stream:    entry.Stream,
		Line:      entry.Line,
		Timestamp: time.Unix(entry.Timestamp, 0),
	})
}

func (p *Pipeline) JobStatus(agentID string, status ufp.JobStatus) {
	release, err := p.store.GetRelease(status.JobID)
	if err != nil {
		return
	}

	switch status.Stage {
	case ufp.StageBuild:
		p.onBuildStatus(release, status)
	case ufp.StageRelease:
		p.onReleaseStatus(release, agentID, status)
	}
}

func (p *Pipeline) onBuildStatus(release *models.Release, status ufp.JobStatus) {
	switch status.Status {
	case ufp.StatusFailed:
		p.failRelease(release, status.Message)

	case ufp.StatusSuccess:
		release.Image = status.Image
		release.Images = status.Images
		release.Digest = status.Digest
		if status.Commit != "" {
			release.Commit = status.Commit
		}
		release.Status = models.StatusReleasing
		p.store.UpdateRelease(release)

		project, err := p.store.GetProject(release.Project)
		if err != nil {
			p.failRelease(release, "project was removed mid-release")
			return
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

	outcome := models.StatusSucceeded
	if status.Status == ufp.StatusFailed {
		outcome = models.StatusFailed
	}

	p.finishTarget(release.ID, agentID, p.agentName(agentID), outcome, status.Message)
	p.settle(release.ID)
}

func (p *Pipeline) finishTarget(releaseID, agentID, agentName string, status models.Status, message string) {
	ended := time.Now()
	p.store.SaveReleaseTarget(&models.ReleaseTarget{
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

	succeeded, failed := 0, 0
	for _, target := range release.Targets {
		if !target.Status.Done() {
			return
		}
		switch target.Status {
		case models.StatusSucceeded:
			succeeded++
		case models.StatusFailed:
			failed++
		}
	}

	if failed > 0 || succeeded == 0 {
		message := "no runner accepted the release"
		if failed > 0 {
			message = "one or more runners failed"
		}
		p.failRelease(release, message)
		return
	}

	p.completeRelease(release, models.StatusSucceeded, "")
	logger.Info("[PIPELINE] release %s: %s live on %d runner(s)", release.ID, release.Image, succeeded)
}

func (p *Pipeline) failRelease(release *models.Release, message string) {
	p.completeRelease(release, models.StatusFailed, message)
	logger.Error("[PIPELINE] release %s failed: %s", release.ID, message)
}

func (p *Pipeline) completeRelease(release *models.Release, status models.Status, message string) {
	ended := time.Now()
	release.Status = status
	release.Message = message
	release.EndedAt = &ended
	release.Duration = ended.Sub(release.StartedAt).Milliseconds()
	p.store.UpdateRelease(release)
}

func (p *Pipeline) agentName(agentID string) string {
	agent, err := p.store.GetAgent(agentID)
	if err != nil {
		return agentID
	}
	return agent.Name
}
