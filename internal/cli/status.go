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

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"uruflow.com/internal/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system status",
	Long:  `Display overall UruFlow system status including repositories, deployments, and services.`,
	Run:   showStatus,
}

// Initialize status command
func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("detailed", false, "Show detailed status information")
}

// Show system status
func showStatus(cmd *cobra.Command, args []string) {
	detailed, _ := cmd.Flags().GetBool("detailed")

	fmt.Printf(" UruFlow System Status\n")
	fmt.Printf("========================\n\n")

	showConfigurationStatus()
	fmt.Printf("\n")

	showRepositoriesStatus(detailed)
	fmt.Printf("\n")

	showDeploymentsStatus()
	fmt.Printf("\n")

	showSSHStatusSummary()
	fmt.Printf("\n")

	showDockerStatus(detailed)

	if detailed {
		fmt.Printf("\n")
		showEnvironmentStatus()
	}
}

func showConfigurationStatus() {
	configPath := config.GetConfigPath(envManager)
	fmt.Printf("  Configuration:\n")

	if fileExists(configPath) {
		fmt.Printf("    Config file: %s\n", configPath)
	} else {
		fmt.Printf("    Config file: %s (not found)\n", configPath)
	}

	fmt.Printf("    Work dir: %s\n", cfg.Settings.WorkDir)
	fmt.Printf("    Max concurrent: %d\n", cfg.Settings.MaxConcurrent)
	fmt.Printf("    Webhook port: %s\n", cfg.Webhook.Port)
}

func showRepositoriesStatus(detailed bool) {
	repos := repositoryService.ListRepositories()
	info := repositoryService.GetRepositoryInfo()

	fmt.Printf(" Repositories (%d total):\n", len(repos))

	if len(repos) == 0 {
		fmt.Printf("    No repositories configured\n")
		return
	}

	readyCount := 0
	for _, repo := range repos {
		if repoInfo, exists := info[repo.Name]; exists {
			if repoInfoMap, ok := repoInfo.(map[string]interface{}); ok {
				if status, ok := repoInfoMap["status"].(map[string]string); ok {
					allReady := true
					for _, branchStatus := range status {
						if branchStatus != "ready" {
							allReady = false
							break
						}
					}
					if allReady {
						readyCount++
					}

					if detailed {
						fmt.Printf("    %s:\n", repo.Name)
						for branch, branchStatus := range status {
							statusEmoji := getStatusEmoji(branchStatus)
							fmt.Printf("      %s %s: %s\n", statusEmoji, branch, branchStatus)
						}
					}
				}
			}
		}
	}

	if !detailed {
		fmt.Printf("    Ready: %d/%d\n", readyCount, len(repos))
		if readyCount < len(repos) {
			fmt.Printf("    Use 'uruflow repo status' for details\n")
		}
	}
}

func showDeploymentsStatus() {
	stats := deploymentService.GetDeploymentStats()
	activeJobs := deploymentService.GetActiveJobs()

	fmt.Printf(" Deployments:\n")
	fmt.Printf("   Active jobs: %d\n", len(activeJobs))
	fmt.Printf("   Queue size: %d/%d\n", stats["queue_size"], stats["queue_capacity"])
	fmt.Printf("   Max workers: %d\n", stats["max_workers"])

	if len(activeJobs) > 0 {
		fmt.Printf("   Currently running:\n")
		for _, job := range activeJobs {
			fmt.Printf("      %s\n", job)
		}
	}
}

func showSSHStatusSummary() {
	fmt.Printf(" SSH:\n")
	if gitService.IsSSHAvailable() {
		if err := gitService.TestSSHConnection(); err != nil {
			fmt.Printf("    Configured but connection failed\n")
		} else {
			fmt.Printf("   Configured and ready\n")
		}
	} else {
		fmt.Printf("    Not configured\n")
	}
}

func showDockerStatus(detailed bool) {
	fmt.Printf(" Docker:\n")

	// Try to get Docker status
	status, err := dockerService.GetStatusOutput()
	if err != nil {
		fmt.Printf("    Docker not available: %v\n", err)
		return
	}

	lines := strings.Split(status, "\n")
	runningCount := 0
	for _, line := range lines {
		if strings.Contains(line, "Up ") {
			runningCount++
		}
	}

	fmt.Printf("    Docker available\n")
	fmt.Printf("    Running containers: %d\n", runningCount)

	if detailed && status != "" {
		fmt.Printf("   Container details:\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" && !strings.Contains(line, "NAMES") {
				fmt.Printf("     %s\n", line)
			}
		}
	}
}

func showEnvironmentStatus() {
	fmt.Printf(" Environment:\n")
	fmt.Printf("   URUFLOW_CONFIG_DIR: %s\n", envManager.ConfigDir)
	fmt.Printf("   URUFLOW_LOG_DIR: %s\n", envManager.LogDir)
	fmt.Printf("   DEBUG: %s\n", getEnvOrDefault("DEBUG", "false"))
}
