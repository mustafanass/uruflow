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

// Update: clean the status, remove unwanted process, Add emoji for status cli to make it modern

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "📊 Show system status",
	Long:  `Show current deployment status and running containers.`,
	Run:   showStatus,
}

// Initialize status command
func init() {
	rootCmd.AddCommand(statusCmd)
}

// Show system status - simple and focused
func showStatus(cmd *cobra.Command, args []string) {
	fmt.Printf("📊 UruFlow Status\n")
	fmt.Printf("==================\n\n")

	// Show active deployments (most important info)
	showActiveDeployments()

	// Show running containers (what's actually deployed)
	showRunningContainers()

	// Show quick repository summary
	showRepositorySummary()
}

// Show active deployments - the most critical information
func showActiveDeployments() {
	activeJobs := deploymentService.GetActiveJobs()

	if len(activeJobs) > 0 {
		fmt.Printf("⚡ Active Deployments:\n")
		for _, job := range activeJobs {
			fmt.Printf("   🔄 %s\n", job)
		}
		fmt.Printf("\n")
	} else {
		fmt.Printf("✅ No active deployments\n\n")
	}
}

// Show running containers - what's actually deployed and working
func showRunningContainers() {
	fmt.Printf("🐳 Running Containers:\n")

	status, err := dockerService.GetStatusOutput()
	if err != nil {
		fmt.Printf("   ❌ Docker not available: %v\n\n", err)
		return
	}

	if status == "" {
		fmt.Printf("   🔴 No containers running\n\n")
		return
	}

	// Parse and show only running containers with clean formatting
	lines := strings.Split(status, "\n")
	runningCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "CONTAINER ID") && !strings.Contains(line, "NAMES") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				containerName := parts[len(parts)-1] // Last field is the name
				status := "🟢 Running"
				if !strings.Contains(line, "Up") {
					status = "🔴 Stopped"
				}
				fmt.Printf("   %s %s\n", status, containerName)
				if strings.Contains(line, "Up") {
					runningCount++
				}
			}
		}
	}

	if runningCount > 0 {
		fmt.Printf("   📈 Total running: %d\n\n", runningCount)
	} else {
		fmt.Printf("   🔴 No containers running\n\n")
	}
}

// Show quick repository summary - just what matters
func showRepositorySummary() {
	repos := repositoryService.ListRepositories()

	if len(repos) == 0 {
		fmt.Printf("📁 Repositories: None configured\n")
		return
	}

	fmt.Printf("📁 Repositories: %d configured\n", len(repos))
	info := repositoryService.GetRepositoryInfo()
	hasIssues := false
	for _, repo := range repos {
		if repoInfo, exists := info[repo.Name]; exists {
			if repoInfoMap, ok := repoInfo.(map[string]interface{}); ok {
				if status, ok := repoInfoMap["status"].(map[string]string); ok {
					for branch, branchStatus := range status {
						if branchStatus != "ready" {
							if !hasIssues {
								fmt.Printf("   ⚠️ Issues found:\n")
								hasIssues = true
							}
							statusEmoji := getStatusEmoji(branchStatus)
							fmt.Printf("      %s %s:%s (%s)\n", statusEmoji, repo.Name, branch, branchStatus)
						}
					}
				}
			}
		}
	}

	if !hasIssues {
		fmt.Printf("   ✅ All repositories ready\n")
	}
}
