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
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"uruflow.com/helper"
	"uruflow.com/internal/models"
	"uruflow.com/internal/utils"
)

type GitService struct {
	logger    *utils.Logger
	sshHelper *helper.SSHHelper
	gitMutex  sync.Mutex
}

func NewGitService(logger *utils.Logger) *GitService {
	return &GitService{
		logger:    logger,
		sshHelper: helper.NewSSHHelper(logger),
	}
}

// Initialize sets up Git service with SSH and safety configuration
func (g *GitService) Initialize() error {
	g.logger.Info("Initializing Git service...")

	if err := g.configureGitSafety(); err != nil {
		g.logger.Warning("Failed to configure Git safety: %v", err)
	}

	g.logger.Info("Checking SSH setup...")
	if err := g.sshHelper.EnsureSSHKey(); err != nil {
		return fmt.Errorf("SSH setup failed: %v", err)
	}

	g.logger.Info("Testing SSH connection...")
	if err := g.sshHelper.TestGitHubConnection(); err != nil {
		g.logger.Warning("SSH connection test failed: %v", err)
		return fmt.Errorf("SSH connection test failed: %v", err)
	}
	g.logger.Success("Git service initialized with SSH authentication")
	return nil
}

// configureGitSafety configures Git to handle ownership issues
func (g *GitService) configureGitSafety() error {
	g.gitMutex.Lock()
	defer g.gitMutex.Unlock()

	cmd := exec.Command("git", "config", "--global", "safe.directory", "*")
	if err := cmd.Run(); err != nil {
		g.logger.Warning("Failed to set global safe directory wildcard: %v", err)
	}
	// specific directory and sub-directory patterns that are used by Uruflow system
	workDirPatterns := []string{
		"/var/uruflow/repositories",
		"/var/uruflow/repositories/*",
		"/var/uruflow/repositories/*/*",
		"/var/uruflow/repositories/*/*/*",
	}

	for _, pattern := range workDirPatterns {
		cmd = exec.Command("git", "config", "--global", "--add", "safe.directory", pattern)
		if err := cmd.Run(); err != nil {
			g.logger.Warning("Failed to add safe directory pattern %s: %v", pattern, err)
		}
	}
	cmd = exec.Command("git", "config", "--global", "safe.directory", "*")
	if err := cmd.Run(); err != nil {
		g.logger.Warning("Failed to disable dubious ownership detection: %v", err)
	}

	g.logger.Success("Git safety configuration completed")
	return nil
}

// ensureRepositorySafety ensures a specific repository path is safe (optimized version)
func (g *GitService) ensureRepositorySafety(repoPath string) error {
	if g.hasAppliedSafety(repoPath) {
		return nil
	}
	cmd := exec.Command("git", "config", "--global", "safe.directory", "*")
	cmd.Run()
	if os.Getuid() == 0 {
		parentDir := filepath.Dir(repoPath)
		grandParentDir := filepath.Dir(parentDir)
		for _, dir := range []string{grandParentDir, parentDir, repoPath} {
			if _, err := os.Stat(dir); err == nil {
				exec.Command("chown", "-R", "root:root", dir).Run()
			}
		}
	}
	g.markSafetyApplied(repoPath)
	return nil
}

var appliedSafety = make(map[string]bool)
var safetyMutex sync.Mutex

func (g *GitService) hasAppliedSafety(repoPath string) bool {
	safetyMutex.Lock()
	defer safetyMutex.Unlock()
	return appliedSafety[repoPath]
}

func (g *GitService) markSafetyApplied(repoPath string) {
	safetyMutex.Lock()
	defer safetyMutex.Unlock()
	appliedSafety[repoPath] = true
}

// fixRepositoryOwnership fixes ownership issues in Docker containers
func (gs *GitService) fixRepositoryOwnership(repoPath string) error {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		parentDir := filepath.Dir(repoPath)
		if _, err := os.Stat(parentDir); err == nil {
			cmd := exec.Command("chown", "-R", "root:root", parentDir)
			if err := cmd.Run(); err != nil {
				gs.logger.Warning("Failed to fix parent directory ownership: %v", err)
			}
		}
		return nil
	}

	gs.logger.Git("Fixing ownership for: %s", repoPath)
	cmd := exec.Command("chown", "-R", "root:root", repoPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fix ownership: %v", err)
	}

	gs.logger.Git("Fixed ownership for repository: %s", repoPath)
	return nil
}

// executeGitCommand executes a git command with minimal overhead
func (gs *GitService) executeGitCommand(args []string, workDir string, env []string) error {
	gitEnv := gs.sshHelper.GetGitEnvironment()
	if env != nil {
		gitEnv = append(gitEnv, env...)
	}
	gitEnv = append(gitEnv, "GIT_CONFIG_GLOBAL=/dev/null")

	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	cmd.Env = gitEnv

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "dubious ownership") {
			gs.ensureRepositorySafety(workDir)
			cmd = exec.Command("git", args...)
			cmd.Dir = workDir
			cmd.Env = gitEnv
			output, err = cmd.CombinedOutput()
		}

		if err != nil {
			return fmt.Errorf("git command failed: %v, output: %s", err, output)
		}
	}

	return nil
}

// SetupRepository clones or updates a repository
func (gs *GitService) SetupRepository(repo models.Repository, branch, repoPath string) error {
	gs.ensureRepositorySafety(repoPath)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		gs.logger.Git("Cloning %s:%s", repo.Name, branch)
		return gs.cloneRepository(repo, branch, repoPath)
	}

	gs.logger.Git("Updating %s:%s", repo.Name, branch)
	return gs.updateRepository(repo, branch, repoPath)
}

// cloneRepository clones a new repository with safety handling
func (gs *GitService) cloneRepository(repo models.Repository, branch, repoPath string) error {
	parentDir := filepath.Dir(repoPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	gs.ensureRepositorySafety(parentDir)
	gs.logger.Git("Cloning repository %s:%s to %s", repo.Name, branch, repoPath)
	cmd := exec.Command("git", "clone", "-b", branch, "--depth", "1", repo.GitURL, repoPath)
	cmd.Env = gs.sshHelper.GetGitEnvironment()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %v, output: %s", err, output)
	}

	gs.ensureRepositorySafety(repoPath)

	gs.logger.Success("Cloned %s:%s successfully", repo.Name, branch)
	return nil
}

// updateRepository updates an existing repository efficiently
func (gs *GitService) updateRepository(repo models.Repository, branch, repoPath string) error {
	gitEnv := gs.sshHelper.GetGitEnvironment()

	if err := gs.executeGitCommand([]string{"fetch", "origin", branch}, repoPath, gitEnv); err != nil {
		return fmt.Errorf("fetch failed: %v", err)
	}

	resetArgs := []string{"reset", "--hard", fmt.Sprintf("origin/%s", branch)}
	if err := gs.executeGitCommand(resetArgs, repoPath, gitEnv); err != nil {
		return fmt.Errorf("reset failed: %v", err)
	}

	gs.executeGitCommand([]string{"clean", "-fd"}, repoPath, gitEnv) // Best effort cleanup

	gs.logger.Success("Updated %s:%s successfully", repo.Name, branch)
	return nil
}
func (gs *GitService) IsSSHAvailable() bool {
	return gs.sshHelper.IsReady()
}

func (gs *GitService) TestSSHConnection() error {
	return gs.sshHelper.TestGitHubConnection()
}

// GetRepositoryInfo returns basic repository information with safety handling
func (gs *GitService) GetRepositoryInfo(repoPath string) (map[string]string, error) {
	gs.ensureRepositorySafety(repoPath)
	info := make(map[string]string)
	gitEnv := gs.sshHelper.GetGitEnvironment()
	if err := gs.executeGitCommand([]string{"rev-parse", "HEAD"}, repoPath, gitEnv); err == nil {
		cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
		cmd.Env = gitEnv
		if output, err := cmd.Output(); err == nil {
			info["commit_hash"] = strings.TrimSpace(string(output))
		}
	}
	if err := gs.executeGitCommand([]string{"branch", "--show-current"}, repoPath, gitEnv); err == nil {
		cmd := exec.Command("git", "-C", repoPath, "branch", "--show-current")
		cmd.Env = gitEnv
		if output, err := cmd.Output(); err == nil {
			info["current_branch"] = strings.TrimSpace(string(output))
		}
	}
	if err := gs.executeGitCommand([]string{"remote", "get-url", "origin"}, repoPath, gitEnv); err == nil {
		cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
		cmd.Env = gitEnv
		if output, err := cmd.Output(); err == nil {
			info["remote_url"] = strings.TrimSpace(string(output))
		}
	}

	return info, nil
}

// ValidateRepository checks if a repository path contains a valid Git repository
func (g *GitService) ValidateRepository(repoPath string) error {
	g.ensureRepositorySafety(repoPath)

	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a valid Git repository: %s", repoPath)
	}
	return g.executeGitCommand([]string{"status", "--porcelain"}, repoPath, nil)
}

// CleanupRepository removes a repository directory safely
func (g *GitService) CleanupRepository(repoPath string) error {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil // Already doesn't exist
	}

	g.logger.Git("Cleaning up repository: %s", repoPath)

	// Fix ownership before removal (important in Docker)
	if os.Getuid() == 0 {
		g.fixRepositoryOwnership(repoPath)
	}

	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("failed to remove repository: %v", err)
	}

	g.logger.Success("Repository cleaned up: %s", repoPath)
	return nil
}
