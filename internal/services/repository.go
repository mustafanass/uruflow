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
		repoPath := rs.getRepositoryPath(repo.Name, branch)

		if err := rs.gitService.SetupRepository(repo, branch, repoPath); err != nil {
			return fmt.Errorf("failed to setup %s:%s - %v", repo.Name, branch, err)
		}

		if err := rs.verifyDockerCompose(repo, repoPath); err != nil {
			rs.logger.Warning("Docker compose verification failed for %s:%s - %v", repo.Name, branch, err)
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

	rs.logger.Info("Found docker-compose file: %s", repo.ComposeFile)
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
		repoPath := rs.getRepositoryPath(repo.Name, branch)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			status[branch] = "not_cloned"
		} else {
			composeFile := filepath.Join(repoPath, repo.ComposeFile)
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				status[branch] = "missing_compose"
			} else {
				status[branch] = "ready"
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