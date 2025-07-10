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
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"uruflow.com/internal/config"
	"uruflow.com/internal/handlers"
	"uruflow.com/internal/models"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "🌐 Start the webhook server",
	Long:  `Start the UruFlow webhook server to listen for Git webhooks and auto-deploy.`,
	Run:   runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringP("port", "p", "", "Port to run server on (overrides config)")
}

// runServer starts the webhook server
func runServer(cmd *cobra.Command, args []string) {
	logger.Startup("Starting UruFlow Auto-Deploy System...")

	port, _ := cmd.Flags().GetString("port")
	if port != "" {
		cfg.Webhook.Port = port
		logger.Info("Port moved to: %s", port)
	}

	config.WatchConfig(envManager, func(newConfig *models.Config) {
		logger.Config("Configuration file changed, reloading...")
		repositoryService.UpdateConfig(newConfig)
		cfg = newConfig
		logger.Success("Configuration reloaded successfully")
	})

	if cfg.Settings.AutoClone {
		logger.Info("Initializing repositories...")
		if err := repositoryService.InitializeRepositories(); err != nil {
			logger.Error("Failed to initialize repositories: %v", err)
			logger.Info("Continuing without repository initialization...")
		}
	}

	server := setupHTTPServer()
	setupGracefulShutdown(server)

	logger.Deploy("UruFlow webhook server started on port %s", cfg.Webhook.Port)
	logger.Info("Webhook endpoint: http://0.0.0.0:%s%s", cfg.Webhook.Port, cfg.Webhook.Path)
	logger.Info("Managing %d repositories", len(cfg.Repositories))

	if gitService.IsSSHAvailable() {
		logger.Success("SSH authentication is configured and ready")
	} else {
		logger.Warning("SSH authentication not configured - some operations may fail")
	}

	logger.Info("Server is ready to receive webhooks from GitHub/GitLab")
	logger.Info("Press Ctrl+C to stop the server")

	address := "0.0.0.0:" + cfg.Webhook.Port
	logger.Info("Starting server on %s", address)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed to start: %v", err)
	}
}

// setupHTTPServer configures and returns the HTTP server
func setupHTTPServer() *http.Server {
	r := mux.NewRouter()
	webhookHandler := handlers.NewWebhookHandler(cfg, repositoryService, deploymentService, gitService, dockerService, logger)

	r.HandleFunc(cfg.Webhook.Path, webhookHandler.HandleWebhook).Methods("POST")
	r.HandleFunc("/health", handleHealth).Methods("GET")
	r.HandleFunc("/status", handleStatus).Methods("GET")

	return &http.Server{
		Addr:         "0.0.0.0:" + cfg.Webhook.Port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// setupGracefulShutdown handles graceful server shutdown
func setupGracefulShutdown(server *http.Server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("Received shutdown signal, gracefully shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		logger.Info("Shutting down deployment service...")
		if err := deploymentService.Shutdown(25 * time.Second); err != nil {
			logger.Warning("Deployment service shutdown error: %v", err)
		}
		logger.Info("Shutting down HTTP server...")
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Server shutdown error: %v", err)
		} else {
			logger.Success("Server shutdown complete")
		}

		logger.Close()
		os.Exit(0)
	}()
}

// handleHealth provides a health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	logger.Info("Health check request from %s", r.RemoteAddr)

	stats := deploymentService.GetDeploymentStats()
	activeJobs := deploymentService.GetActiveJobs()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "healthy",
		"active_jobs": len(activeJobs),
		"queue_size":  stats["queue_size"],
		"timestamp":   time.Now().Unix(),
	})
}

// handleStatus provides a detailed status endpoint
func handleStatus(w http.ResponseWriter, r *http.Request) {
	logger.Info("Status request from %s", r.RemoteAddr)

	stats := deploymentService.GetDeploymentStats()
	activeJobs := deploymentService.GetActiveJobs()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status":             "running",
		"active_jobs":        stats["active_jobs"],
		"queue_size":         stats["queue_size"],
		"max_workers":        stats["max_workers"],
		"total_jobs":         stats["total_jobs"],
		"completed_jobs":     stats["completed_jobs"],
		"failed_jobs":        stats["failed_jobs"],
		"timeout_jobs":       stats["timeout_jobs"],
		"success_rate":       stats["success_rate"],
		"active_job_details": activeJobs,
		"repositories":       len(cfg.Repositories),
		"ssh_available":      gitService.IsSSHAvailable(),
		"timestamp":          time.Now().Unix(),
	}

	json.NewEncoder(w).Encode(response)
}
