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
	"time"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy [repository] [branch]",
	Short: "🚀 Deploy a repository manually",
	Long:  `Manually trigger deployment of a specific repository and branch.`,
	Args:  cobra.ExactArgs(2),
	Run:   runDeploy,
}

var deployStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "📊 Show deployment status",
	Long:  `Show current deployment queue and jobs.`,
	Run:   showDeployStatus,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.AddCommand(deployStatusCmd)
	deployCmd.Flags().BoolP("force", "f", false, "Force deployment even if containers are running")
}

func runDeploy(cmd *cobra.Command, args []string) {
	repoName := args[0]
	branch := args[1]

	fmt.Printf("🚀 Starting Manual deployment: %s:%s\n", repoName, branch)
	logger.Info("Manual deployment requested: %s:%s", repoName, branch)

	repo := repositoryService.GetRepository(repoName)
	if repo == nil {
		logger.Error("Repository '%s' not found or disabled", repoName)
		fmt.Printf("❌ Repository '%s' not found or disabled\n", repoName)
		fmt.Printf("\n📦 Available repositories:\n")
		for _, r := range repositoryService.ListRepositories() {
			fmt.Printf("  - %s (branches: %v)\n", r.Name, r.Branches)
		}
		return
	}

	if !repositoryService.IsBranchConfigured(repo, branch) {
		logger.Error("Branch '%s' not configured for repository '%s'", branch, repoName)
		fmt.Printf("❌ Branch '%s' not configured for repository '%s'\n", branch, repoName)
		fmt.Printf("🌿 Available branches for %s: %v\n", repoName, repo.Branches)
		return
	}
	startTime := time.Now()
	fmt.Printf("⚡ Executing deployment...\n")

	if err := deploymentService.DeployDirect(*repo, branch); err != nil {
		duration := time.Since(startTime)
		logger.Error("Deployment failed: %v", err)
		fmt.Printf("❌ Deployment failed after %v: %v\n", duration.Round(time.Second), err)
		return
	}

	duration := time.Since(startTime)
	logger.Success("Deployment completed successfully for %s:%s (took %v)", repoName, branch, duration.Round(time.Second))
	fmt.Printf("✅ Deployment completed successfully for %s:%s (took %v)\n", repoName, branch, duration.Round(time.Second))

	fmt.Printf("\n🔍 Checking deployed containers...\n")
	showDeployedContainers()
}

// showDeployedContainers displays information about deployed containers
func showDeployedContainers() {
	status, err := dockerService.GetStatusOutput()
	if err != nil {
		fmt.Printf("❌ Could not get container status: %v\n", err)
		return
	}

	if status == "" {
		fmt.Printf("🔴 No containers found\n")
		return
	}

	fmt.Printf("%s\n", status)
}

func showDeployStatus(cmd *cobra.Command, args []string) {
	stats := deploymentService.GetDeploymentStats()
	activeJobs := deploymentService.GetActiveJobs()

	fmt.Printf("📊 Deployment Status\n")
	fmt.Printf("==================\n\n")

	fmt.Printf("⚙️ Max Workers: %d\n", stats["max_workers"])
	fmt.Printf("⚡ Active Jobs: %d\n", stats["active_jobs"])

	if len(activeJobs) > 0 {
		fmt.Printf("\n🔄 Currently Running:\n")
		for _, job := range activeJobs {
			fmt.Printf("  - %s\n", job)
		}
	} else {
		fmt.Printf("\n💤 No active deployments\n")
	}

	fmt.Printf("\n📦 Current Docker Containers:\n")
	status, err := dockerService.GetStatusOutput()
	if err != nil {
		fmt.Printf("❌ Could not get container status: %v\n", err)
	} else if status == "" {
		fmt.Printf("🔴 No containers running\n")
	} else {
		fmt.Printf("%s\n", status)
	}
}
