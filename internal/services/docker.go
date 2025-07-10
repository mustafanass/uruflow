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
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"uruflow.com/internal/models"
	"uruflow.com/internal/utils"
)

// DockerService handles Docker Compose operations
type DockerService struct {
	logger         *utils.Logger
	composeCommand string
}

// NewDockerService creates a new Docker service
func NewDockerService(logger *utils.Logger) *DockerService {
	ds := &DockerService{
		logger: logger,
	}

	ds.composeCommand = ds.detectComposeCommand()
	logger.Info("Using Docker Compose command: %s", ds.composeCommand)

	return ds
}

// detectComposeCommand detects whether to use 'docker compose' or 'docker-compose'
func (d *DockerService) detectComposeCommand() string {
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err == nil {
		return "docker compose"
	}

	cmd = exec.Command("docker-compose", "version")
	if err := cmd.Run(); err == nil {
		return "docker-compose"
	}

	d.logger.Warning("Neither 'docker compose' nor 'docker-compose' found, defaulting to 'docker compose'")
	return "docker compose"
}

// Deploy deploys services using Docker Compose
func (d *DockerService) Deploy(repo models.Repository, branch, repoPath string) ([]string, error) {
	projectName := d.getProjectName(repo, branch)
	d.logger.Docker("Starting deployment for %s:%s using %s (project: %s)", repo.Name, branch, d.composeCommand, projectName)
	d.logger.Docker("Stopping any existing services...")
	if err := d.stopServices(repo.ComposeFile, projectName, repoPath); err != nil {
		d.logger.Warning("Failed to stop existing services (this may be normal): %v", err)
	}
	d.logger.Docker("Starting services with conflict resolution...")
	if err := d.startServices(repo.ComposeFile, projectName, repoPath); err != nil {
		d.logger.Error("Service startup failed: %v", err)
		return nil, err
	}

	// Get list of deployed services
	services, err := d.getServices(repo.ComposeFile, projectName, repoPath)
	if err != nil {
		d.logger.Warning("Could not get services list: %v", err)
		// Don't fail deployment just because we can't list services
		return []string{"unknown"}, nil
	}

	d.logger.Success("Successfully deployed %d services for %s:%s: %v", len(services), repo.Name, branch, services)
	return services, nil
}

// DeployWithContext deploys services using Docker Compose with context support
func (d *DockerService) DeployWithContext(ctx context.Context, repo models.Repository, branch, repoPath string) ([]string, error) {
	return d.Deploy(repo, branch, repoPath)
}

// getProjectName returns the Docker Compose project name
func (d *DockerService) getProjectName(repo models.Repository, branch string) string {
	if branchConfig, exists := repo.BranchConfig[branch]; exists && branchConfig.ProjectName != "" {
		return branchConfig.ProjectName
	}
	return fmt.Sprintf("%s-%s", repo.Name, branch)
}

// stopServices stops existing Docker Compose services with enhanced cleanup
func (d *DockerService) stopServices(composeFile, projectName, workDir string) error {
	d.logger.Docker("Stopping existing services for project: %s", projectName)
	args := d.buildComposeArgs(composeFile, projectName, "down", "--remove-orphans")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		d.logger.Warning("Normal stop failed, trying force removal: %v", err)
		d.logger.Warning("Output: %s", string(output))
		if cleanupErr := d.aggressiveProjectCleanup(projectName, composeFile, workDir); cleanupErr != nil {
			d.logger.Warning("Aggressive cleanup also failed: %v", cleanupErr)
		}
	} else {
		d.logger.Docker("Successfully stopped services for: %s", projectName)
	}

	return nil
}

// aggressiveProjectCleanup performs comprehensive project cleanup
func (d *DockerService) aggressiveProjectCleanup(projectName, composeFile, workDir string) error {
	d.logger.Warning("Performing aggressive project cleanup for: %s", projectName)
	containers, err := d.getProjectContainers(projectName, composeFile, workDir)
	if err != nil {
		d.logger.Warning("Failed to get project containers via compose: %v", err)
	}
	for _, container := range containers {
		d.logger.Docker("Force removing project container: %s", container)
		removeCmd := exec.Command("docker", "rm", "-f", container)
		if removeErr := removeCmd.Run(); removeErr != nil {
			d.logger.Warning("Failed to remove container %s: %v", container, removeErr)
		} else {
			d.logger.Success("Removed container: %s", container)
		}
	}
	d.logger.Docker("Cleaning up containers by name pattern...")
	return d.cleanupContainersByPattern(projectName)
}

// getProjectContainers gets containers for a specific docker-compose project
func (d *DockerService) getProjectContainers(projectName, composeFile, workDir string) ([]string, error) {
	args := d.buildComposeArgs(composeFile, projectName, "ps", "-q")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			containers = append(containers, strings.TrimSpace(line))
		}
	}

	return containers, nil
}

// cleanupContainersByPattern removes containers matching project name patterns
func (d *DockerService) cleanupContainersByPattern(projectName string) error {
	d.logger.Docker("Cleaning up containers by pattern for project: %s", projectName)
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}
	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, container := range containers {
		container = strings.TrimSpace(container)
		if container != "" && (strings.Contains(container, projectName) || strings.HasPrefix(container, projectName)) {
			d.logger.Docker("Removing container by pattern: %s", container)
			removeCmd := exec.Command("docker", "rm", "-f", container)
			if removeErr := removeCmd.Run(); removeErr != nil {
				d.logger.Warning("Failed to remove container %s: %v", container, removeErr)
			} else {
				d.logger.Success("Removed container: %s", container)
			}
		}
	}

	return nil
}

// startServices starts Docker Compose services with enhanced conflict resolution
func (d *DockerService) startServices(composeFile, projectName, workDir string) error {
	d.logger.Docker("Starting services for project: %s", projectName)
	d.logger.Docker("Performing proactive cleanup...")
	if cleanupErr := d.cleanupContainersByPattern(projectName); cleanupErr != nil {
		d.logger.Warning("Proactive cleanup failed: %v", cleanupErr)
	}
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		d.logger.Docker("Attempt %d/%d: Starting services...", attempt, maxRetries)

		args := d.buildComposeArgs(composeFile, projectName, "up", "-d", "--build", "--force-recreate", "--remove-orphans")
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workDir
		cmd.Env = append(cmd.Env, fmt.Sprintf("COMPOSE_PROJECT_NAME=%s", projectName))
		output, err := cmd.CombinedOutput()
		if err == nil {
			d.logger.Success("Successfully started services for: %s", projectName)
			return nil
		}
		d.logger.Warning("Attempt %d failed: %v", attempt, err)
		d.logger.Warning("Output: %s", string(output))

		outputStr := string(output)
		if strings.Contains(outputStr, "already in use") ||
			strings.Contains(outputStr, "Conflict") ||
			strings.Contains(outputStr, "container name") ||
			strings.Contains(outputStr, "is already in use by container") {

			d.logger.Warning("Container conflict detected on attempt %d, performing aggressive cleanup...", attempt)
			if aggressiveErr := d.aggressiveContainerCleanup(projectName, outputStr); aggressiveErr != nil {
				d.logger.Warning("Aggressive cleanup failed: %v", aggressiveErr)
			}
			if attempt < maxRetries {
				d.logger.Docker("Waiting 3 seconds before retry...")
				time.Sleep(3 * time.Second)
			}
		} else {
			return fmt.Errorf("docker compose up failed: %v, output: %s", err, output)
		}
	}
	return fmt.Errorf("failed to start services after %d attempts", maxRetries)
}

// aggressiveContainerCleanup performs targeted container removal based on error analysis
func (d *DockerService) aggressiveContainerCleanup(projectName string, outputStr string) error {
	d.logger.Warning("Performing aggressive container cleanup...")
	containerName := d.extractConflictingContainerName(outputStr)

	if containerName != "" {
		d.logger.Warning("Removing specific conflicting container: %s", containerName)
		removeCmd := exec.Command("docker", "rm", "-f", containerName)
		if removeErr := removeCmd.Run(); removeErr != nil {
			d.logger.Warning("Failed to remove specific container %s: %v", containerName, removeErr)
		} else {
			d.logger.Success("Successfully removed conflicting container: %s", containerName)
			return nil
		}
	}
	d.logger.Warning("Falling back to pattern-based cleanup for project: %s", projectName)
	if err := d.cleanupContainersByPattern(projectName); err != nil {
		d.logger.Warning("Pattern-based cleanup failed: %v", err)
	}
	d.logger.Warning("Attempting cleanup of containers with similar names...")
	return d.cleanupSimilarContainers(projectName)
}

// extractConflictingContainerName extracts container name from Docker error messages
func (d *DockerService) extractConflictingContainerName(outputStr string) string {
	d.logger.Docker("Analyzing error output for container name extraction...")
	if idx := strings.Index(outputStr, "The container name"); idx != -1 {
		remaining := outputStr[idx:]
		if start := strings.Index(remaining, `"/`); start != -1 {
			nameStart := start + 2 // Skip `"/`
			if end := strings.Index(remaining[nameStart:], `"`); end != -1 {
				containerName := remaining[nameStart : nameStart+end]
				d.logger.Docker("Extracted container name from pattern 1: %s", containerName)
				return containerName
			}
		}
	}
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Container") && (strings.Contains(line, "Creating") || strings.Contains(line, "Error")) {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Container" && i+1 < len(parts) {
					containerName := parts[i+1]
					d.logger.Docker("Extracted container name from pattern 2: %s", containerName)
					return containerName
				}
			}
		}
	}
	if start := strings.Index(outputStr, `"/`); start != -1 {
		nameStart := start + 2
		if end := strings.Index(outputStr[nameStart:], `"`); end != -1 {
			containerName := outputStr[nameStart : nameStart+end]
			d.logger.Docker("Extracted container name from pattern 3: %s", containerName)
			return containerName
		}
	}
	if idx := strings.Index(outputStr, "already in use by container"); idx != -1 {
		remaining := outputStr[idx:]
		if start := strings.Index(remaining, `"`); start != -1 {
			nameStart := start + 1
			if end := strings.Index(remaining[nameStart:], `"`); end != -1 {
				containerID := remaining[nameStart : nameStart+end]
				// Try to get container name from ID
				if containerName := d.getContainerNameFromID(containerID); containerName != "" {
					d.logger.Docker("Extracted container name from ID: %s -> %s", containerID, containerName)
					return containerName
				}
			}
		}
	}
	d.logger.Warning("Could not extract container name from error output")
	return ""
}

// getContainerNameFromID converts container ID to container name
func (d *DockerService) getContainerNameFromID(containerID string) string {
	cmd := exec.Command("docker", "inspect", "--format", "{{.Name}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	name := strings.TrimSpace(string(output))
	if strings.HasPrefix(name, "/") {
		name = name[1:]
	}
	return name
}

// cleanupSimilarContainers removes containers with names similar to the project
func (d *DockerService) cleanupSimilarContainers(projectName string) error {
	d.logger.Docker("Cleaning up containers with similar names to: %s", projectName)
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	projectParts := strings.Split(projectName, "-")

	for _, container := range containers {
		container = strings.TrimSpace(container)
		if container == "" {
			continue
		}
		shouldRemove := false
		for _, part := range projectParts {
			if part != "" && strings.Contains(container, part) {
				shouldRemove = true
				break
			}
		}

		if shouldRemove {
			d.logger.Docker("Removing similar container: %s", container)
			removeCmd := exec.Command("docker", "rm", "-f", container)
			if removeErr := removeCmd.Run(); removeErr != nil {
				d.logger.Warning("Failed to remove similar container %s: %v", container, removeErr)
			} else {
				d.logger.Success("Removed similar container: %s", container)
			}
		}
	}

	return nil
}

// getServices returns the list of services
func (d *DockerService) getServices(composeFile, projectName, workDir string) ([]string, error) {
	args := d.buildComposeArgs(composeFile, projectName, "ps", "--services")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	services := strings.Split(strings.TrimSpace(string(output)), "\n")
	var filteredServices []string
	for _, service := range services {
		if strings.TrimSpace(service) != "" {
			filteredServices = append(filteredServices, strings.TrimSpace(service))
		}
	}

	return filteredServices, nil
}

// buildComposeArgs builds the command arguments for Docker Compose
func (d *DockerService) buildComposeArgs(composeFile, projectName string, subcommands ...string) []string {
	var args []string

	if d.composeCommand == "docker compose" {
		args = []string{"docker", "compose", "-f", composeFile, "-p", projectName}
	} else {
		args = []string{"docker-compose", "-f", composeFile, "-p", projectName}
	}

	args = append(args, subcommands...)
	return args
}

// GetStatusOutput returns formatted container status
func (d *DockerService) GetStatusOutput() (string, error) {
	cmd := exec.Command("docker", "ps", "--format",
		"table {{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.Image}}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// cleanupConflictingContainers removes containers that might be causing name conflicts (legacy method)
func (d *DockerService) cleanupConflictingContainers(projectName string) error {
	d.logger.Docker("Legacy cleanup: Cleaning up conflicting containers for project: %s", projectName)
	cmd := exec.Command("docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s", projectName), "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, container := range containers {
		if container != "" {
			d.logger.Docker("Removing conflicting container: %s", container)
			removeCmd := exec.Command("docker", "rm", "-f", container)
			if removeErr := removeCmd.Run(); removeErr != nil {
				d.logger.Warning("Failed to remove container %s: %v", container, removeErr)
			}
		}
	}

	return nil
}

// Cleanup removes unused Docker resources (only if cleanup_enabled is true)
func (d *DockerService) Cleanup() error {
	d.logger.Info("Starting Docker cleanup...")

	cmd := exec.Command("docker", "container", "prune", "-f")
	if err := cmd.Run(); err != nil {
		d.logger.Warning("Failed to cleanup containers: %v", err)
	}

	cmd = exec.Command("docker", "image", "prune", "-f")
	if err := cmd.Run(); err != nil {
		d.logger.Warning("Failed to cleanup images: %v", err)
	}

	cmd = exec.Command("docker", "volume", "prune", "-f")
	if err := cmd.Run(); err != nil {
		d.logger.Warning("Failed to cleanup volumes: %v", err)
	}

	d.logger.Success("Docker cleanup completed")
	return nil
}
