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
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Repository management commands",
	Long:  `Manage repositories: list, info, update, initialize, and check status.`,
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all repositories",
	Long:  `List all configured repositories with their status.`,
	Run:   listRepositories,
}

var repoInfoCmd = &cobra.Command{
	Use:   "info [repository]",
	Short: "Show repository information",
	Long:  `Show detailed information about a specific repository.`,
	Args:  cobra.MaximumNArgs(1),
	Run:   showRepositoryInfo,
}

var repoUpdateCmd = &cobra.Command{
	Use:   "update [repository]",
	Short: "Update repository",
	Long:  `Update a specific repository by pulling latest changes.`,
	Args:  cobra.ExactArgs(1),
	Run:   updateRepository,
}

var repoInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize all repositories",
	Long:  `Clone/initialize all configured repositories.`,
	Run:   initializeRepositories,
}

var repoStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show repository status",
	Long:  `Show the status of all repositories (cloned, ready, missing compose, etc.).`,
	Run:   showRepositoryStatus,
}

func init() {
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoInfoCmd)
	repoCmd.AddCommand(repoUpdateCmd)
	repoCmd.AddCommand(repoInitCmd)
	repoCmd.AddCommand(repoStatusCmd)
}

// listRepositories displays all configured repositories with their basic information and status
func listRepositories(cmd *cobra.Command, args []string) {
	repos := repositoryService.ListRepositories()

	if len(repos) == 0 {
		fmt.Printf(" No repositories configured\n")
		return
	}
	fmt.Printf(" Configured Repositories (%d)\n", len(repos))
	fmt.Printf("===========================\n\n")
	for _, repo := range repos {
		status := "🟢"
		if !repo.Enabled {
			status = "🔴"
		}

		fmt.Printf("%s %s\n", status, repo.Name)
		fmt.Printf("   URL: %s\n", repo.GitURL)
		fmt.Printf("   Branches: %s\n", strings.Join(repo.Branches, ", "))
		fmt.Printf("   Auto-deploy: %t\n", repo.AutoDeploy)
		fmt.Printf("   Compose file: %s\n", repo.ComposeFile)
		fmt.Printf("\n")
	}
}

// showRepositoryInfo displays detailed information for a specific repository or all repositories if no argument provided
func showRepositoryInfo(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		info := repositoryService.GetRepositoryInfo()

		fmt.Printf(" Repository Information\n")
		fmt.Printf("========================\n\n")

		for name, repoInfo := range info {
			fmt.Printf("  %s\n", name)
			if repoInfoMap, ok := repoInfo.(map[string]interface{}); ok {
				fmt.Printf("   Git URL: %s\n", repoInfoMap["git_url"])
				fmt.Printf("   Branches: %v\n", repoInfoMap["branches"])
				fmt.Printf("   Auto-deploy: %t\n", repoInfoMap["auto_deploy"])
				fmt.Printf("   Compose file: %s\n", repoInfoMap["compose_file"])

				if status, ok := repoInfoMap["status"].(map[string]string); ok {
					fmt.Printf("   Status:\n")
					for branch, branchStatus := range status {
						statusEmoji := getStatusEmoji(branchStatus)
						fmt.Printf("     %s %s: %s\n", statusEmoji, branch, branchStatus)
					}
				}
			}
			fmt.Printf("\n")
		}
		return
	}
	repoName := args[0]
	repo := repositoryService.GetRepository(repoName)
	if repo == nil {
		logger.Error("Repository '%s' not found", repoName)
		return
	}

	fmt.Printf(" Repository: %s\n", repo.Name)
	fmt.Printf("================\n\n")
	fmt.Printf("Git URL: %s\n", repo.GitURL)
	fmt.Printf("Branches: %s\n", strings.Join(repo.Branches, ", "))
	fmt.Printf("Auto-deploy: %t\n", repo.AutoDeploy)
	fmt.Printf("Enabled: %t\n", repo.Enabled)
	fmt.Printf("Compose file: %s\n", repo.ComposeFile)

	if len(repo.BranchConfig) > 0 {
		fmt.Printf("\nBranch Configuration:\n")
		for branch, config := range repo.BranchConfig {
			fmt.Printf("  %s:\n", branch)
			fmt.Printf("    Project name: %s\n", config.ProjectName)
		}
	}
}

// updateRepository pulls the latest changes for a specific repository
func updateRepository(cmd *cobra.Command, args []string) {
	repoName := args[0]

	logger.Info("Updating repository: %s", repoName)

	if err := repositoryService.UpdateRepository(repoName); err != nil {
		logger.Error("Failed to update repository: %v", err)
		return
	}

	logger.Success("Repository %s updated successfully", repoName)
	fmt.Printf(" Repository %s updated successfully\n", repoName)
}

// initializeRepositories clones and sets up all configured repositories
func initializeRepositories(cmd *cobra.Command, args []string) {
	logger.Info("Initializing all repositories...")

	if err := repositoryService.InitializeRepositories(); err != nil {
		logger.Error("Failed to initialize repositories: %v", err)
		return
	}

	logger.Success("All repositories initialized successfully")
	fmt.Printf(" All repositories initialized successfully\n")
}

// showRepositoryStatus displays the current status of all repositories and their branches
func showRepositoryStatus(cmd *cobra.Command, args []string) {
	info := repositoryService.GetRepositoryInfo()

	fmt.Printf(" Repository Status\n")
	fmt.Printf("===================\n\n")
	for name, repoInfo := range info {
		fmt.Printf("  %s\n", name)
		if repoInfoMap, ok := repoInfo.(map[string]interface{}); ok {
			if status, ok := repoInfoMap["status"].(map[string]string); ok {
				for branch, branchStatus := range status {
					statusEmoji := getStatusEmoji(branchStatus)
					fmt.Printf("   %s %s: %s\n", statusEmoji, branch, branchStatus)
				}
			}
		}
		fmt.Printf("\n")
	}
}

// Add simple emoji for status
func getStatusEmoji(status string) string {
	switch status {
	case "ready":
		return "✅"
	case "not_cloned":
		return "❌"
	case "missing_compose":
		return "⚠️"
	default:
		return "❓"
	}
}
