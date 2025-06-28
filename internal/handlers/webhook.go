package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"uruflow.com/internal/models"
	"uruflow.com/internal/services"
	"uruflow.com/internal/utils"
)

// WebhookHandler handles GitHub webhook requests
type WebhookHandler struct {
	config            *models.Config
	repositoryService *services.RepositoryService
	deploymentService *services.DeploymentService
	gitService        *services.GitService
	dockerService     *services.DockerService
	logger            *utils.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(
	config *models.Config,
	repositoryService *services.RepositoryService,
	deploymentService *services.DeploymentService,
	gitService *services.GitService,
	dockerService *services.DockerService,
	logger *utils.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		config:            config,
		repositoryService: repositoryService,
		deploymentService: deploymentService,
		gitService:        gitService,
		dockerService:     dockerService,
		logger:            logger,
	}
}

// HandleWebhook processes incoming webhook requests using direct deployment
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("=== WEBHOOK REQUEST START ===")
	h.logger.Info("Webhook request received from %s", r.RemoteAddr)

	if r.Method != http.MethodPost {
		h.logger.Warning("Invalid method: %s (expected POST)", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	var webhook models.GitHubWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.Error("Error decoding webhook JSON: %v", err)
		http.Error(w, "Bad request - invalid JSON", http.StatusBadRequest)
		return
	}

	branch := strings.TrimPrefix(webhook.Ref, "refs/heads/")

	if !strings.HasPrefix(webhook.Ref, "refs/heads/") {
		h.logger.Info("Ignoring non-branch ref: %s", webhook.Ref)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Ignoring non-branch ref: %s", webhook.Ref)
		return
	}

	h.logger.Webhook("Processing webhook: %s:%s (commit: %s)",
		webhook.Repository.Name, branch, webhook.HeadCommit.ID[:7])

	repo := h.repositoryService.GetRepository(webhook.Repository.Name)
	if repo == nil {
		h.logger.Error("Repository '%s' not found in configuration", webhook.Repository.Name)
		http.Error(w, fmt.Sprintf("Repository '%s' not configured", webhook.Repository.Name), http.StatusNotFound)
		return
	}

	if !repo.AutoDeploy {
		h.logger.Info("Auto-deploy disabled for repository %s", repo.Name)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Auto-deploy disabled for repository %s", repo.Name)
		return
	}

	if !h.repositoryService.IsBranchConfigured(repo, branch) {
		h.logger.Info("Branch '%s' not configured for deployment in repository '%s'", branch, repo.Name)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Branch '%s' not configured for deployment", branch)
		return
	}

	h.logger.Success("All checks passed, starting direct deployment")

	if !h.gitService.IsSSHAvailable() {
		h.logger.Error("SSH authentication not available for webhook deployment")
		http.Error(w, "SSH authentication not configured", http.StatusInternalServerError)
		return
	}

	repoPath := filepath.Join(h.config.Settings.WorkDir, repo.Name, branch)
	h.logger.Webhook("Verifying SSH connection before deployment")

	maxRetries := 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		h.logger.Webhook("SSH connection test attempt %d/%d", attempt, maxRetries)
		if err := h.gitService.TestSSHConnection(); err != nil {
			lastErr = err
			h.logger.Warning("SSH test attempt %d failed: %v", attempt, err)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
		} else {
			h.logger.Success("SSH connection verified for webhook deployment")
			lastErr = nil
			break
		}
	}

	if lastErr != nil {
		h.logger.Error("SSH connection test failed for webhook after %d attempts: %v", maxRetries, lastErr)
		http.Error(w, fmt.Sprintf("SSH connection failed: %v", lastErr), http.StatusInternalServerError)
		return
	}

	h.logger.Webhook("Applying Git safety fixes for %s:%s", repo.Name, branch)
	if err := h.applyGitSafetyFixes(repoPath); err != nil {
		h.logger.Warning("Failed to apply Git safety fixes: %v", err)
	}

	startTime := time.Now()
	h.logger.Webhook("Starting direct deployment for %s:%s", repo.Name, branch)

	if err := h.deploymentService.DeployDirect(*repo, branch); err != nil {
		duration := time.Since(startTime)
		h.logger.Error("Webhook deployment failed after %v: %v", duration.Round(time.Second), err)
		http.Error(w, fmt.Sprintf("Deployment failed: %v", err), http.StatusInternalServerError)
		return
	}

	duration := time.Since(startTime)
	h.logger.Success("Webhook deployment completed successfully for %s:%s (took %v)", repo.Name, branch, duration.Round(time.Second))
	w.WriteHeader(http.StatusOK)
	response := fmt.Sprintf("Deployment completed successfully for %s:%s (commit: %s) in %v",
		repo.Name, branch, webhook.HeadCommit.ID[:7], duration.Round(time.Second))
	fmt.Fprintf(w, response)

	h.logger.Info("=== WEBHOOK REQUEST END ===")
}

// applyGitSafetyFixes applies Git safety configurations specifically for webhook context
func (h *WebhookHandler) applyGitSafetyFixes(repoPath string) error {
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "unknown"
	}

	h.logger.Webhook("Applying aggressive Git safety fixes as user: %s (UID: %d)", currentUser, os.Getuid())

	// Set global wildcard to trust all directories
	cmd := exec.Command("git", "config", "--global", "safe.directory", "*")
	if err := cmd.Run(); err != nil {
		h.logger.Warning("Failed to set global safe directory wildcard: %v", err)
	}

	// Add comprehensive list of safe directories
	parentDir := filepath.Dir(repoPath)
	grandParentDir := filepath.Dir(parentDir)
	workDir := h.config.Settings.WorkDir

	safePaths := []string{
		repoPath,
		parentDir,
		grandParentDir,
		workDir,
		"/var/uruflow/repositories",
		"/var/uruflow/repositories/*",
		"/var/uruflow/repositories/*/*",
		"/var/uruflow/repositories/*/*/*",
		repoPath + "/*",
		parentDir + "/*",
		workDir + "/*",
	}

	for _, path := range safePaths {
		cmd = exec.Command("git", "config", "--global", "--add", "safe.directory", path)
		if err := cmd.Run(); err != nil {
			h.logger.Warning("Failed to add safe directory %s: %v", path, err)
		}
	}

	// Always fix ownership aggressively when running as root
	if os.Getuid() == 0 {
		h.logger.Webhook("Running as root, applying aggressive ownership fixes")
		dirsToFix := []string{grandParentDir, parentDir, repoPath, workDir}
		for _, dir := range dirsToFix {
			if err := h.fixOwnershipAsRoot(dir, dir); err != nil {
				h.logger.Warning("Failed to fix ownership for %s: %v", dir, err)
			}
		}
	} else {
		h.logger.Webhook("Running as non-root user (%s), ensuring permissions for: %s", currentUser, repoPath)
		if err := h.ensureUserPermissions(repoPath, parentDir, currentUser); err != nil {
			h.logger.Warning("Failed to ensure user permissions: %v", err)
		}
	}

	h.logger.Webhook("Aggressive Git safety fixes applied for %s", repoPath)
	return nil
}

// fixOwnershipAsRoot fixes ownership when running as root
func (h *WebhookHandler) fixOwnershipAsRoot(repoPath, parentDir string) error {
	// Fix repository ownership
	if _, err := os.Stat(repoPath); err == nil {
		cmd := exec.Command("chown", "-R", "root:root", repoPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to fix repository ownership: %v", err)
		}
	}

	// Fix parent directory ownership
	if _, err := os.Stat(parentDir); err == nil {
		cmd := exec.Command("chown", "-R", "root:root", parentDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to fix parent directory ownership: %v", err)
		}
	}

	return nil
}

func (h *WebhookHandler) ensureUserPermissions(repoPath, parentDir, currentUser string) error {

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		h.logger.Warning("Failed to create parent directory: %v", err)
	}

	if currentUser == "uruflow" {
		if fileInfo, err := os.Stat(parentDir); err == nil {
			if _, ok := fileInfo.Sys().(*os.ProcessState); ok {
				h.logger.Webhook("Directory ownership check for %s", parentDir)
			}
		}

		cmd := exec.Command("sudo", "chown", "-R", "uruflow:uruflow", parentDir)
		if err := cmd.Run(); err != nil {
			h.logger.Warning("Could not fix ownership with sudo (this may be expected): %v", err)

			if err := os.Chmod(parentDir, 0755); err != nil {
				h.logger.Warning("Failed to set directory permissions: %v", err)
			}
		}
	}

	return nil
}
