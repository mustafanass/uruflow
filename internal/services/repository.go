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

// Update: add new IsRepositoryInitialized methods for dynamic Initialized repo when deployed,
// cleanupCorruptedRepository to Handles corrupted repositories by removing them before re-initialization
// better error handling and also update other methods to fit the deployment
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"uruflow.com/internal/models"
	"uruflow.com/internal/utils"
)

// RepositoryService manages repository operations
type RepositoryService struct {
	config     *models.Config
	gitService *GitService
	logger     *utils.Logger
}

// NewRepositoryService creates a new repository service
func NewRepositoryService(config *models.Config, gitService *GitService, logger *utils.Logger) *RepositoryService {
	return &RepositoryService{
		config:     config,
		gitService: gitService,
		logger:     logger,
	}
}

// UpdateConfig updates the configuration reference
func (rs *RepositoryService) UpdateConfig(config *models.Config) {
	rs.config = config
	rs.logger.Config("Repository service configuration updated")
}

// IsRepositoryInitialized checks if a repository branch is properly initialized
func (rs *RepositoryService) IsRepositoryInitialized(repoName, branch string) bool {
	repo := rs.GetRepository(repoName)
	if repo == nil {
		rs.logger.Warning("Repository %s not found or disabled", repoName)
		return false
	}

	if !rs.IsBranchConfigured(repo, branch) {
		rs.logger.Warning("Branch %s not configured for repository %s", branch, repoName)
		return false
	}

	repoPath := rs.getRepositoryPath(repoName, branch)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		rs.logger.Debug("Repository directory missing: %s", repoPath)
		return false
	}

	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		rs.logger.Debug("Git directory missing: %s", gitDir)
		return false
	}

	composeFile := filepath.Join(repoPath, repo.ComposeFile)
	if fileInfo, err := os.Stat(composeFile); os.IsNotExist(err) {
		rs.logger.Debug("Docker compose file missing: %s", composeFile)
		return false
	} else if fileInfo.Size() == 0 {
		rs.logger.Debug("Docker compose file is empty: %s", composeFile)
		return false
	}

	rs.logger.Debug("Repository %s:%s is properly initialized", repoName, branch)
	return true
}

// InitializeRepository initializes a specific repository and branch
func (rs *RepositoryService) InitializeRepository(repo models.Repository, branch string) error {
	if err := rs.ValidateRepository(repo); err != nil {
		return fmt.Errorf("repository validation failed: %v", err)
	}

	if !rs.IsBranchConfigured(&repo, branch) {
		return fmt.Errorf("branch %s is not configured for repository %s", branch, repo.Name)
	}

	repoPath := rs.getRepositoryPath(repo.Name, branch)

	rs.logger.Info("Initializing repository %s:%s at %s", repo.Name, branch, repoPath)

	if err := rs.cleanupCorruptedRepository(repoPath); err != nil {
		rs.logger.Warning("Failed to cleanup existing repository: %v", err)
	}

	if err := rs.gitService.SetupRepository(repo, branch, repoPath); err != nil {
		return fmt.Errorf("failed to setup repository %s:%s - %v", repo.Name, branch, err)
	}

	if err := rs.verifyDockerCompose(repo, repoPath); err != nil {
		return fmt.Errorf("docker compose verification failed for %s:%s - %v", repo.Name, branch, err)
	}

	rs.logger.Success("Repository %s:%s initialized successfully", repo.Name, branch)
	return nil
}

// cleanupCorruptedRepository removes potentially corrupted repository
func (rs *RepositoryService) cleanupCorruptedRepository(repoPath string) error {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil
	}

	rs.logger.Debug("Cleaning up existing repository: %s", repoPath)
	return os.RemoveAll(repoPath)
}

// InitializeRepositories clones all configured repositories
func (rs *RepositoryService) InitializeRepositories() error {
	rs.logger.Info("Initializing repositories...")

	if rs.config.Settings.AutoClone {
		for _, repo := range rs.config.Repositories {
			if !repo.Enabled {
				rs.logger.Info("Skipping disabled repository: %s", repo.Name)
				continue
			}

			if err := rs.cloneRepository(repo); err != nil {
				rs.logger.Error("Failed to initialize repository %s: %v", repo.Name, err)
				return err
			}
		}
	}

	rs.logger.Success("All repositories initialized successfully")
	return nil
}

// cloneRepository clones a repository and its configured branches
func (rs *RepositoryService) cloneRepository(repo models.Repository) error {
	rs.logger.Info("Initializing repository: %s", repo.Name)

	for _, branch := range repo.Branches {
		if err := rs.InitializeRepository(repo, branch); err != nil {
			return err
		}
	}

	rs.logger.Success("Repository %s initialized with %d branches", repo.Name, len(repo.Branches))
	return nil
}

// verifyDockerCompose checks if docker-compose.yml exists in the repository
func (rs *RepositoryService) verifyDockerCompose(repo models.Repository, repoPath string) error {
	composeFile := filepath.Join(repoPath, repo.ComposeFile)

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose file not found: %s", repo.ComposeFile)
	}

	if fileInfo, err := os.Stat(composeFile); err == nil {
		if fileInfo.Size() == 0 {
			return fmt.Errorf("docker-compose file is empty: %s", repo.ComposeFile)
		}
	}

	rs.logger.Debug("Verified docker-compose file: %s", repo.ComposeFile)
	return nil
}

// getRepositoryPath returns the local path for a repository branch
func (rs *RepositoryService) getRepositoryPath(repoName, branch string) string {
	return filepath.Join(rs.config.Settings.WorkDir, repoName, branch)
}

// GetRepository finds a repository by name
func (rs *RepositoryService) GetRepository(name string) *models.Repository {
	for _, repo := range rs.config.Repositories {
		if repo.Name == name && repo.Enabled {
			return &repo
		}
	}
	return nil
}

// ListRepositories returns all enabled repositories
func (rs *RepositoryService) ListRepositories() []models.Repository {
	var repos []models.Repository
	for _, repo := range rs.config.Repositories {
		if repo.Enabled {
			repos = append(repos, repo)
		}
	}
	return repos
}

// IsBranchConfigured checks if a branch is configured for deployment
func (rs *RepositoryService) IsBranchConfigured(repo *models.Repository, branch string) bool {
	for _, b := range repo.Branches {
		if b == branch {
			return true
		}
	}
	return false
}

// GetRepositoryInfo returns detailed information about repositories
func (rs *RepositoryService) GetRepositoryInfo() map[string]interface{} {
	info := make(map[string]interface{})

	for _, repo := range rs.config.Repositories {
		if !repo.Enabled {
			continue
		}

		repoInfo := map[string]interface{}{
			"name":         repo.Name,
			"git_url":      repo.GitURL,
			"branches":     repo.Branches,
			"auto_deploy":  repo.AutoDeploy,
			"compose_file": repo.ComposeFile,
			"status":       rs.getRepositoryStatus(repo),
		}

		info[repo.Name] = repoInfo
	}

	return info
}

// getRepositoryStatus checks the status of a repository
func (rs *RepositoryService) getRepositoryStatus(repo models.Repository) map[string]string {
	status := make(map[string]string)

	for _, branch := range repo.Branches {
		if rs.IsRepositoryInitialized(repo.Name, branch) {
			status[branch] = "ready"
		} else {
			repoPath := rs.getRepositoryPath(repo.Name, branch)
			if _, err := os.Stat(repoPath); os.IsNotExist(err) {
				status[branch] = "not_cloned"
			} else {
				status[branch] = "missing_compose"
			}
		}
	}

	return status
}

// ValidateRepository validates repository configuration
func (rs *RepositoryService) ValidateRepository(repo models.Repository) error {
	if repo.Name == "" {
		return fmt.Errorf("repository name is required")
	}

	if repo.GitURL == "" {
		return fmt.Errorf("git URL is required for repository %s", repo.Name)
	}

	if len(repo.Branches) == 0 {
		return fmt.Errorf("at least one branch is required for repository %s", repo.Name)
	}

	if !strings.HasPrefix(repo.GitURL, "http") && !strings.HasPrefix(repo.GitURL, "git@") {
		return fmt.Errorf("invalid git URL format for repository %s", repo.Name)
	}

	if repo.ComposeFile == "" {
		return fmt.Errorf("compose file is required for repository %s", repo.Name)
	}

	return nil
}

// UpdateRepository updates an existing repository
func (rs *RepositoryService) UpdateRepository(repoName string) error {
	repo := rs.GetRepository(repoName)
	if repo == nil {
		return fmt.Errorf("repository %s not found or disabled", repoName)
	}

	rs.logger.Info("Updating repository: %s", repoName)
	for _, branch := range repo.Branches {
		repoPath := rs.getRepositoryPath(repo.Name, branch)

		if err := rs.gitService.SetupRepository(*repo, branch, repoPath); err != nil {
			return fmt.Errorf("failed to update %s:%s - %v", repo.Name, branch, err)
		}
	}

	rs.logger.Success("Repository %s updated successfully", repoName)
	return nil
}

// ForceReinitializeRepository forces re-initialization of a repository branch
func (rs *RepositoryService) ForceReinitializeRepository(repoName, branch string) error {
	repo := rs.GetRepository(repoName)
	if repo == nil {
		return fmt.Errorf("repository %s not found or disabled", repoName)
	}

	rs.logger.Info("Force re-initializing repository %s:%s", repoName, branch)
	repoPath := rs.getRepositoryPath(repoName, branch)
	if err := rs.cleanupCorruptedRepository(repoPath); err != nil {
		return fmt.Errorf("failed to cleanup repository: %v", err)
	}

	return rs.InitializeRepository(*repo, branch)
}
