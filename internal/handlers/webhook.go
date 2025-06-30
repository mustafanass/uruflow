package handlers

import (
	"context"
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
	w.Header().Set("Content-Type", "application/json")
	
	var response string
	var statusCode int = http.StatusOK
	
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Panic in webhook handler: %v", r)
			statusCode = http.StatusInternalServerError
			response = `{"error": "Internal server error", "status": "failed"}`
		}

		w.WriteHeader(statusCode)
		if response == "" {
			response = `{"status": "ok", "message": "Request processed"}`
		}
		fmt.Fprint(w, response)
		
		h.logger.Info("=== WEBHOOK REQUEST END (Status: %d) ===", statusCode)
	}()

	h.logger.Info("=== WEBHOOK REQUEST START ===")
	h.logger.Info("Webhook request received from %s", r.RemoteAddr)
	deployCtx, deployCancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer deployCancel()
	
	if r.Method != http.MethodPost {
		h.logger.Warning("Invalid method: %s (expected POST)", r.Method)
		statusCode = http.StatusMethodNotAllowed
		response = `{"error": "Method not allowed", "status": "failed"}`
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Error reading request body: %v", err)
		statusCode = http.StatusBadRequest
		response = `{"error": "Error reading request body", "status": "failed"}`
		return
	}
	defer r.Body.Close()

	var webhook models.GitHubWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.Error("Error decoding webhook JSON: %v", err)
		statusCode = http.StatusBadRequest
		response = `{"error": "Bad request - invalid JSON", "status": "failed"}`
		return
	}

	branch := strings.TrimPrefix(webhook.Ref, "refs/heads/")

	if !strings.HasPrefix(webhook.Ref, "refs/heads/") {
		h.logger.Info("Ignoring non-branch ref: %s", webhook.Ref)
		response = fmt.Sprintf(`{"status": "ignored", "message": "Ignoring non-branch ref: %s"}`, webhook.Ref)
		return
	}

	h.logger.Webhook("Processing webhook: %s:%s (commit: %s)",
		webhook.Repository.Name, branch, webhook.HeadCommit.ID[:7])

	repo := h.repositoryService.GetRepository(webhook.Repository.Name)
	if repo == nil {
		h.logger.Error("Repository '%s' not found in configuration", webhook.Repository.Name)
		statusCode = http.StatusNotFound
		response = fmt.Sprintf(`{"error": "Repository '%s' not configured", "status": "failed"}`, webhook.Repository.Name)
		return
	}

	if !repo.AutoDeploy {
		h.logger.Info("Auto-deploy disabled for repository %s", repo.Name)
		response = fmt.Sprintf(`{"status": "disabled", "message": "Auto-deploy disabled for repository %s"}`, repo.Name)
		return
	}

	if !h.repositoryService.IsBranchConfigured(repo, branch) {
		h.logger.Info("Branch '%s' not configured for deployment in repository '%s'", branch, repo.Name)
		response = fmt.Sprintf(`{"status": "ignored", "message": "Branch '%s' not configured for deployment"}`, branch)
		return
	}

	h.logger.Success("All checks passed, starting direct deployment")

	select {
	case <-deployCtx.Done():
		h.logger.Error("Deployment context cancelled during validation")
		statusCode = http.StatusRequestTimeout
		response = `{"error": "Deployment timeout during validation", "status": "failed"}`
		return
	default:
	}

	if !h.gitService.IsSSHAvailable() {
		h.logger.Error("SSH authentication not available for webhook deployment")
		statusCode = http.StatusInternalServerError
		response = `{"error": "SSH authentication not configured", "status": "failed"}`
		return
	}

	repoPath := filepath.Join(h.config.Settings.WorkDir, repo.Name, branch)
	h.logger.Webhook("Verifying SSH connection before deployment")

	if err := h.testSSHWithContext(deployCtx); err != nil {
		h.logger.Error("SSH connection test failed: %v", err)
		statusCode = http.StatusInternalServerError
		response = fmt.Sprintf(`{"error": "SSH connection failed: %v", "status": "failed"}`, err)
		return
	}

	h.logger.Webhook("Applying Git safety fixes for %s:%s", repo.Name, branch)
	if err := h.applyGitSafetyFixes(repoPath); err != nil {
		h.logger.Warning("Failed to apply Git safety fixes: %v", err)
	}

	startTime := time.Now()
	h.logger.Webhook("Starting direct deployment for %s:%s", repo.Name, branch)
	if err := h.deployWithContext(deployCtx, *repo, branch); err != nil {
		duration := time.Since(startTime)
		h.logger.Error("Webhook deployment failed after %v: %v", duration.Round(time.Second), err)
		statusCode = http.StatusInternalServerError
		response = fmt.Sprintf(`{"error": "Deployment failed: %v", "status": "failed", "duration": "%v"}`, err, duration.Round(time.Second))
		return
	}

	duration := time.Since(startTime)
	h.logger.Success("Webhook deployment completed successfully for %s:%s (took %v)", repo.Name, branch, duration.Round(time.Second))
	
	response = fmt.Sprintf(`{"status": "success", "message": "Deployment completed successfully", "repository": "%s", "branch": "%s", "commit": "%s", "duration": "%v"}`,
		repo.Name, branch, webhook.HeadCommit.ID[:7], duration.Round(time.Second))
}

// testSSHWithContext tests SSH connection with context timeout
func (h *WebhookHandler) testSSHWithContext(ctx context.Context) error {
	maxRetries := 3
	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("SSH test cancelled: %v", ctx.Err())
		default:
		}
		
		h.logger.Webhook("SSH connection test attempt %d/%d", attempt, maxRetries)
		if err := h.gitService.TestSSHConnection(); err != nil {
			lastErr = err
			h.logger.Warning("SSH test attempt %d failed: %v", attempt, err)
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return fmt.Errorf("SSH test cancelled during retry: %v", ctx.Err())
				case <-time.After(time.Duration(attempt) * time.Second):
					continue
				}
			}
		} else {
			h.logger.Success("SSH connection verified for webhook deployment")
			return nil
		}
	}
	
	return fmt.Errorf("SSH connection test failed after %d attempts: %v", maxRetries, lastErr)
}

// deployWithContext deploys with context timeout
func (h *WebhookHandler) deployWithContext(ctx context.Context, repo models.Repository, branch string) error {
	resultChan := make(chan error, 1)

	go func() {
		defer close(resultChan)
		err := h.deploymentService.DeployDirect(repo, branch)
		resultChan <- err
	}()
	
	select {
	case <-ctx.Done():
		return fmt.Errorf("deployment cancelled: %v", ctx.Err())
	case err := <-resultChan:
		return err
	}
}

// applyGitSafetyFixes applies Git safety configurations specifically for webhook context
func (h *WebhookHandler) applyGitSafetyFixes(repoPath string) error {
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "unknown"
	}

	h.logger.Webhook("Applying Git safety fixes as user: %s (UID: %d)", currentUser, os.Getuid())

	cmd := exec.Command("git", "config", "--global", "safe.directory", "*")
	if err := cmd.Run(); err != nil {
		h.logger.Warning("Failed to set global safe directory wildcard: %v", err)
	}

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
	if _, err := os.Stat(repoPath); err == nil {
		cmd := exec.Command("chown", "-R", "root:root", repoPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to fix repository ownership: %v", err)
		}
	}
	
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
