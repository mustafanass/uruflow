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
	"github.com/spf13/cobra"
	"os"
	"uruflow.com/env_manager"
	"uruflow.com/internal/config"
	"uruflow.com/internal/models"
	"uruflow.com/internal/services"
	"uruflow.com/internal/utils"
)

var (
	envManager        *env_manager.EnvManager
	logger            *utils.Logger
	cfg               *models.Config
	gitService        *services.GitService
	dockerService     *services.DockerService
	repositoryService *services.RepositoryService
	deploymentService *services.DeploymentService
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "uruflow",
	Short: "🚀 UruFlow Auto-Deploy System",
	Long: `UruFlow is a simple and efficient auto-deployment system that watches for Git webhooks
and automatically deploys your applications using Docker Compose.

Examples:
	uruflow                      Start the webhook server (default)
	uruflow server               Start the webhook server  
	uruflow deploy repo main     Deploy a repository manually
	uruflow repo list            List all repositories
	uruflow config info          Show configuration information`,
	PersistentPreRun: initializeServices,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("No command specified, starting webhook server...")
		runServer(cmd, args)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug mode")
	rootCmd.Flags().StringP("port", "p", "", "Port to run server on (overrides config)")
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func initializeServices(cmd *cobra.Command, args []string) {
	logger = utils.NewLogger("[URUFLOW] ")

	debug, _ := cmd.Flags().GetBool("debug")
	if debug {
		os.Setenv("DEBUG", "true")
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		logger.Info("Verbose mode enabled")
	}

	envManager = env_manager.NewEnvManager()

	configPath := config.GetConfigPath(envManager)
	if verbose {
		logger.Config("Using configuration file: %s", configPath)
	}

	var err error
	cfg, err = config.Load(envManager)
	if err != nil {
		logger.Fatal("Failed to load configuration: %v", err)
	}

	gitService = services.NewGitService(logger)
	dockerService = services.NewDockerService(logger)
	repositoryService = services.NewRepositoryService(cfg, gitService, logger)
	deploymentService = services.NewDeploymentService(cfg, repositoryService, gitService, dockerService, logger)

	if verbose {
		logger.Info("Initializing Git service with SSH support...")
	}
	if err := gitService.Initialize(); err != nil {
		if verbose {
			logger.Warning("Git service initialization failed: %v", err)
		}
	} else if verbose {
		logger.Success("SSH authentication configured")
	}
}
