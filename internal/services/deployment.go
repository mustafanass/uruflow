/*
 * Copyright (C) 2025 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of Uruflow, an open-source automation tool.
 *
 * Uruflow is a tool designed to streamline and automate Docker-based deployments.
 *
 * Uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3 of the License.
 *
 * Uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with Uruflow. If not, see <https://www.gnu.org/licenses/>.
 */

package services

// Update: re-design the deployment to make it init the repo when start manual deployment and so on .

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"uruflow.com/internal/models"
	"uruflow.com/internal/utils"
)

// DockerDeployer interface
type DockerDeployer interface {
	Deploy(repo models.Repository, branch string, repoPath string) ([]string, error)
	DeployWithContext(ctx context.Context, repo models.Repository, branch string, repoPath string) ([]string, error)
	Cleanup() error
}

// DeploymentService manages direct deployment with smart auto-initialization
type DeploymentService struct {
	config            *models.Config
	repositoryService *RepositoryService
	gitService        *GitService
	dockerService     DockerDeployer
	activeJobs        map[string]bool
	activeJobsMu      sync.RWMutex
	logger            *utils.Logger
	buildMutex        sync.Mutex
	totalJobs         int64
	completedJobs     int64
	failedJobs        int64
	metricsMu         sync.RWMutex
}

// NewDeploymentService creates a new deployment service with smart auto-initialization
func NewDeploymentService(
	config *models.Config,
	repositoryService *RepositoryService,
	gitService *GitService,
	dockerService DockerDeployer,
	logger *utils.Logger,
) *DeploymentService {
	ds := &DeploymentService{
		config:            config,
		repositoryService: repositoryService,
		gitService:        gitService,
		dockerService:     dockerService,
		activeJobs:        make(map[string]bool),
		logger:            logger,
	}

	ds.logger.Success("Deployment service started with smart auto-initialization")
	return ds
}

// DeployDirect performs direct deployment with smart auto-initialization
func (ds *DeploymentService) DeployDirect(repo models.Repository, branch string) error {
	jobKey := fmt.Sprintf("%s:%s", repo.Name, branch)
	ds.activeJobsMu.Lock()
	if ds.activeJobs[jobKey] {
		ds.activeJobsMu.Unlock()
		return fmt.Errorf("deployment already in progress for %s", jobKey)
	}
	ds.activeJobs[jobKey] = true
	ds.activeJobsMu.Unlock()
	defer func() {
		ds.activeJobsMu.Lock()
		delete(ds.activeJobs, jobKey)
		ds.activeJobsMu.Unlock()
	}()

	startTime := time.Now()
	ds.logger.Deploy("Starting deployment: %s", jobKey)

	if !ds.repositoryService.IsRepositoryInitialized(repo.Name, branch) {
		ds.logger.Info("Repository not initialized, setting up automatically...")
		if err := ds.repositoryService.InitializeRepository(repo, branch); err != nil {
			ds.logger.Error("Auto-initialization failed: %v", err)
			ds.metricsMu.Lock()
			ds.failedJobs++
			ds.metricsMu.Unlock()
			return fmt.Errorf("auto-initialization failed: %v", err)
		}
		ds.logger.Success("Repository %s:%s auto-initialized successfully", repo.Name, branch)
	}

	ds.metricsMu.Lock()
	ds.totalJobs++
	ds.metricsMu.Unlock()

	if err := ds.executeSmartDeployment(repo, branch); err != nil {
		duration := time.Since(startTime)
		ds.logger.Error("Deployment failed after %v: %v", duration.Round(time.Second), err)

		ds.metricsMu.Lock()
		ds.failedJobs++
		ds.metricsMu.Unlock()

		return err
	}

	duration := time.Since(startTime)
	ds.logger.Success("Deployment completed: %s (took %v)", jobKey, duration.Round(time.Second))

	ds.metricsMu.Lock()
	ds.completedJobs++
	ds.metricsMu.Unlock()

	return nil
}

// executeSmartDeployment performs deployment with intelligent repository handling
func (ds *DeploymentService) executeSmartDeployment(repo models.Repository, branch string) error {
	ds.buildMutex.Lock()
	defer ds.buildMutex.Unlock()

	repoPath := filepath.Join(ds.config.Settings.WorkDir, repo.Name, branch)

	// Additional validation: ensure repository is still valid after initialization
	if !ds.repositoryService.IsRepositoryInitialized(repo.Name, branch) {
		return fmt.Errorf("repository validation failed: %s:%s not properly initialized", repo.Name, branch)
	}

	// Update repository to latest changes
	ds.logger.Deploy("Updating repository %s:%s to latest changes", repo.Name, branch)
	if err := ds.gitService.SetupRepository(repo, branch, repoPath); err != nil {
		return fmt.Errorf("repository update failed: %v", err)
	}

	// Verify compose file exists after update
	composeFile := filepath.Join(repoPath, repo.ComposeFile)
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose file not found after update: %s", repo.ComposeFile)
	}
	ds.logger.Deploy("Verified docker-compose file: %s", repo.ComposeFile)

	ds.logger.Deploy("Starting Docker deployment")
	services, err := ds.dockerService.Deploy(repo, branch, repoPath)
	if err != nil {
		return fmt.Errorf("docker deployment failed: %v", err)
	}

	ds.logger.Success("Deployed %d services for %s:%s: %v", len(services), repo.Name, branch, services)

	if ds.config.Settings.CleanupEnabled {
		ds.logger.Deploy("Running cleanup")
		if err := ds.dockerService.Cleanup(); err != nil {
			ds.logger.Warning("Cleanup failed: %v", err)
		}
	}

	return nil
}

// GetActiveJobs returns the current active jobs
func (ds *DeploymentService) GetActiveJobs() []string {
	ds.activeJobsMu.RLock()
	defer ds.activeJobsMu.RUnlock()

	jobs := make([]string, 0, len(ds.activeJobs))
	for job := range ds.activeJobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetDeploymentStats returns deployment statistics
func (ds *DeploymentService) GetDeploymentStats() map[string]interface{} {
	ds.metricsMu.RLock()
	defer ds.metricsMu.RUnlock()

	ds.activeJobsMu.RLock()
	activeCount := len(ds.activeJobs)
	ds.activeJobsMu.RUnlock()

	return map[string]interface{}{
		"queue_size":     0,
		"queue_capacity": 0,
		"max_workers":    ds.config.Settings.MaxConcurrent,
		"active_jobs":    activeCount,
		"total_jobs":     ds.totalJobs,
		"completed_jobs": ds.completedJobs,
		"failed_jobs":    ds.failedJobs,
	}
}

// Shutdown gracefully shuts down the deployment service
func (ds *DeploymentService) Shutdown(timeout time.Duration) error {
	ds.logger.Info("Shutting down deployment service...")
	start := time.Now()
	for time.Since(start) < timeout {
		ds.activeJobsMu.RLock()
		activeCount := len(ds.activeJobs)
		ds.activeJobsMu.RUnlock()
		if activeCount == 0 {
			ds.logger.Success("Deployment service shut down gracefully")
			return nil
		}

		time.Sleep(time.Second)
	}

	ds.logger.Warning("Deployment service shutdown timeout reached")
	return fmt.Errorf("shutdown timeout exceeded")
}
