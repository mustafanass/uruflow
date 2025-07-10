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

package handlers

// Update: re-design the webhook, better error handling advanced internal process , add request id for all request
// add internal helper methods , add support for both github and gitlab based its models you can see more on models
// fixed webhook validations for secret key, and encrypt secret key and decrypt using Sha256 for github, and gitlab only compares tokens directly (based on gitlab design)

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
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

// WebhookResponse represents a standardized webhook response
type WebhookResponse struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	RequestID string                 `json:"request_id,omitempty"`
}

// WebhookHandler handles GitHub/GitLab webhook requests
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

// HandleWebhook processes incoming webhook requests with improved error handling
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := h.generateRequestID()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	response := &WebhookResponse{
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
	}

	defer func() {
		if rec := recover(); rec != nil {
			h.logger.Error("[%s] Error with webhook handler: %v", requestID, rec)
			response.Status = "error"
			response.Error = "Internal server error"
			response.Message = "An unexpected error occurred"
			h.sendResponse(w, http.StatusInternalServerError, response)
		}

		duration := time.Since(startTime)
		h.logger.Info("[%s] === WEBHOOK REQUEST END (Duration: %v, Status: %s) ===",
			requestID, duration.Round(time.Millisecond), response.Status)
	}()

	h.logger.Info("[%s] === WEBHOOK REQUEST START ===", requestID)
	h.logger.Info("[%s] Webhook request from %s", requestID, r.RemoteAddr)
	if r.Method != http.MethodPost {
		h.logger.Warning("[%s] Invalid method: %s (expected POST)", requestID, r.Method)
		response.Status = "failed"
		response.Error = "Method not allowed"
		response.Message = fmt.Sprintf("Invalid HTTP method: %s", r.Method)
		h.sendResponse(w, http.StatusMethodNotAllowed, response)
		return
	}

	body, err := h.readRequestBody(r, requestID)
	if err != nil {
		response.Status = "failed"
		response.Error = "Bad request"
		response.Message = err.Error()
		h.sendResponse(w, http.StatusBadRequest, response)
		return
	}

	if err := h.validateWebhookSecret(r, body, requestID); err != nil {
		response.Status = "failed"
		response.Error = "Unauthorized"
		response.Message = "Webhook signature validation failed"
		h.sendResponse(w, http.StatusUnauthorized, response)
		return
	}

	webhook, err := h.parseWebhook(body, requestID)
	if err != nil {
		response.Status = "failed"
		response.Error = "Invalid payload"
		response.Message = err.Error()
		h.sendResponse(w, http.StatusBadRequest, response)
		return
	}

	branch := strings.TrimPrefix(webhook.Ref, "refs/heads/")

	if err := h.validateWebhook(webhook, branch, requestID); err != nil {
		response.Status = "ignored"
		response.Message = err.Error()
		response.Details = map[string]interface{}{
			"repository": webhook.Repository.Name,
			"ref":        webhook.Ref,
		}
		h.sendResponse(w, http.StatusOK, response)
		return
	}

	pusherInfo := h.getPusherInfo(webhook)
	h.logger.Webhook("[%s] Processing: %s:%s (commit: %s, pusher: %s)",
		requestID, webhook.Repository.Name, branch,
		h.getShortCommitID(webhook.HeadCommit.ID),
		pusherInfo)

	repo, err := h.validateRepository(webhook.Repository.Name, branch, requestID)
	if err != nil {
		response.Status = "failed"
		response.Error = "Configuration error"
		response.Message = err.Error()
		statusCode := http.StatusNotFound
		if strings.Contains(err.Error(), "disabled") {
			statusCode = http.StatusOK
			response.Status = "disabled"
			response.Error = ""
		}
		h.sendResponse(w, statusCode, response)
		return
	}

	if !h.gitService.IsSSHAvailable() {
		h.logger.Error("[%s] SSH authentication not available", requestID)
		response.Status = "failed"
		response.Error = "Configuration error"
		response.Message = "SSH authentication not configured"
		h.sendResponse(w, http.StatusServiceUnavailable, response)
		return
	}

	deploymentDetails, err := h.executeDeployment(repo, branch, webhook, requestID)
	if err != nil {
		response.Status = "failed"
		response.Error = "Deployment failed"
		response.Message = err.Error()
		response.Details = deploymentDetails
		h.sendResponse(w, http.StatusInternalServerError, response)
		return
	}

	response.Status = "success"
	response.Message = "Deployment completed successfully"
	response.Details = deploymentDetails
	h.sendResponse(w, http.StatusOK, response)
}

// validateWebhookSecret validates the webhook secret for both GitHub and GitLab
func (h *WebhookHandler) validateWebhookSecret(r *http.Request, body []byte, requestID string) error {
	secretKey := h.config.Webhook.Secret
	if secretKey == "" {
		h.logger.Warning("[%s] No webhook secret configured - skipping validation", requestID)
		return nil
	}

	githubSignature := r.Header.Get("X-Hub-Signature-256")
	if githubSignature == "" {
		githubSignature = r.Header.Get("X-Hub-Signature")
	}

	gitlabSignature := r.Header.Get("X-Gitlab-Token")
	if githubSignature != "" {
		return h.validateGitHubSignature(githubSignature, body, secretKey, requestID)
	} else if gitlabSignature != "" {
		return h.validateGitLabSignature(gitlabSignature, secretKey, requestID)
	}

	h.logger.Error("[%s] No signature header found in webhook request", requestID)
	return fmt.Errorf("missing webhook signature")
}

// validateGitHubSignature validates GitHub webhook signature
func (h *WebhookHandler) validateGitHubSignature(signature string, body []byte, secret string, requestID string) error {
	var expectedSignature string

	if strings.HasPrefix(signature, "sha256=") {
		h.logger.Debug("[%s] Validating GitHub SHA256 signature", requestID)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expectedSignature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	} else if strings.HasPrefix(signature, "sha1=") {
		h.logger.Debug("[%s] Validating GitHub SHA1 signature (legacy)", requestID)
		mac := hmac.New(sha1.New, []byte(secret))
		mac.Write(body)
		expectedSignature = "sha1=" + hex.EncodeToString(mac.Sum(nil))
	} else {
		h.logger.Error("[%s] Invalid GitHub signature format: %s", requestID, signature)
		return fmt.Errorf("invalid signature format")
	}

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		h.logger.Error("[%s] GitHub signature validation failed", requestID)
		h.logger.Debug("[%s] Expected: %s, Got: %s", requestID, expectedSignature, signature)
		return fmt.Errorf("invalid webhook signature")
	}

	h.logger.Success("[%s] GitHub signature validation passed", requestID)
	return nil
}

// validateGitLabSignature validates GitLab webhook signature
func (h *WebhookHandler) validateGitLabSignature(signature string, secret string, requestID string) error {
	h.logger.Debug("[%s] Validating GitLab token signature", requestID)

	if signature != secret {
		h.logger.Error("[%s] GitLab token validation failed", requestID)
		return fmt.Errorf("invalid webhook token")
	}

	h.logger.Success("[%s] GitLab token validation passed", requestID)
	return nil
}

// readRequestBody reads and validates the request body
func (h *WebhookHandler) readRequestBody(r *http.Request, requestID string) ([]byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, 10<<20)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("[%s] Error reading request body: %v", requestID, err)
		return nil, fmt.Errorf("failed to read request body")
	}
	defer r.Body.Close()

	if len(body) == 0 {
		h.logger.Error("[%s] Empty request body", requestID)
		return nil, fmt.Errorf("empty request body")
	}

	h.logger.Debug("[%s] Request body size: %d bytes", requestID, len(body))
	return body, nil
}

// parseWebhook parses the webhook JSON payload
func (h *WebhookHandler) parseWebhook(body []byte, requestID string) (*models.GitHubWebhook, error) {
	var webhook models.GitHubWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.Error("[%s] Error parsing webhook JSON: %v", requestID, err)
		sample := string(body)
		if len(sample) > 200 {
			sample = sample[:200] + "..."
		}
		h.logger.Debug("[%s] Body sample: %s", requestID, sample)
		return nil, fmt.Errorf("invalid JSON format")
	}

	if webhook.Repository.Name == "" {
		return nil, fmt.Errorf("missing repository name")
	}
	if webhook.Ref == "" {
		return nil, fmt.Errorf("missing ref")
	}

	return &webhook, nil
}

// validateWebhook validates the webhook data
func (h *WebhookHandler) validateWebhook(webhook *models.GitHubWebhook, branch string, requestID string) error {
	if !strings.HasPrefix(webhook.Ref, "refs/heads/") {
		h.logger.Info("[%s] Ignoring non-branch ref: %s", requestID, webhook.Ref)
		return fmt.Errorf("non-branch ref: %s", webhook.Ref)
	}

	if webhook.HeadCommit.ID == "" {
		h.logger.Info("[%s] Ignoring webhook without commits", requestID)
		return fmt.Errorf("no commits in push")
	}

	return nil
}

// validateRepository validates repository and branch configuration
func (h *WebhookHandler) validateRepository(repoName, branch, requestID string) (*models.Repository, error) {
	repo := h.repositoryService.GetRepository(repoName)
	if repo == nil {
		h.logger.Error("[%s] Repository '%s' not found in configuration", requestID, repoName)
		return nil, fmt.Errorf("repository '%s' not configured", repoName)
	}

	if !repo.AutoDeploy {
		h.logger.Info("[%s] Auto-deploy disabled for repository %s", requestID, repo.Name)
		return nil, fmt.Errorf("auto-deploy disabled for repository %s", repo.Name)
	}

	if !h.repositoryService.IsBranchConfigured(repo, branch) {
		h.logger.Info("[%s] Branch '%s' not configured for deployment in repository '%s'",
			requestID, branch, repo.Name)
		return nil, fmt.Errorf("branch '%s' not configured for deployment", branch)
	}

	return repo, nil
}

// executeDeployment performs the actual deployment
func (h *WebhookHandler) executeDeployment(repo *models.Repository, branch string, webhook *models.GitHubWebhook, requestID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	startTime := time.Now()
	h.logger.Webhook("[%s] Starting deployment for %s:%s", requestID, repo.Name, branch)
	if err := h.testSSHConnection(ctx, requestID); err != nil {
		return map[string]interface{}{
			"duration": time.Since(startTime).String(),
			"stage":    "ssh_test",
		}, err
	}

	repoPath := filepath.Join(h.config.Settings.WorkDir, repo.Name, branch)
	if err := h.applyGitSafetyFixes(repoPath, requestID); err != nil {
		h.logger.Warning("[%s] Failed to apply Git safety fixes: %v", requestID, err)
	}

	err := h.deployWithContext(ctx, *repo, branch, requestID)
	duration := time.Since(startTime)

	details := map[string]interface{}{
		"repository": repo.Name,
		"branch":     branch,
		"commit":     h.getShortCommitID(webhook.HeadCommit.ID),
		"duration":   duration.Round(time.Second).String(),
		"timestamp":  startTime.Unix(),
	}

	if webhook.Pusher.Name != "" {
		details["pusher"] = webhook.Pusher.Name
	} else if webhook.Sender.Login != "" {
		details["pusher"] = webhook.Sender.Login
	}

	if webhook.HeadCommit.Author.Name != "" {
		details["author"] = webhook.HeadCommit.Author.Name
	}

	if webhook.HeadCommit.Message != "" {
		details["commit_message"] = h.truncateString(webhook.HeadCommit.Message, 100)
	}

	if err != nil {
		h.logger.Error("[%s] Deployment failed after %v: %v", requestID, duration.Round(time.Second), err)
		details["stage"] = "deployment"
		return details, err
	}

	h.logger.Success("[%s] Deployment completed successfully in %v", requestID, duration.Round(time.Second))
	return details, nil
}

// testSSHConnection tests SSH connection with retries
func (h *WebhookHandler) testSSHConnection(ctx context.Context, requestID string) error {
	maxRetries := 3
	baseDelay := time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("SSH test cancelled: %v", ctx.Err())
		default:
		}

		h.logger.Debug("[%s] SSH connection test attempt %d/%d", requestID, attempt, maxRetries)

		if err := h.gitService.TestSSHConnection(); err == nil {
			h.logger.Success("[%s] SSH connection verified", requestID)
			return nil
		} else {
			h.logger.Warning("[%s] SSH test attempt %d failed: %v", requestID, attempt, err)

			if attempt < maxRetries {
				delay := baseDelay * time.Duration(attempt)
				select {
				case <-ctx.Done():
					return fmt.Errorf("SSH test cancelled during retry: %v", ctx.Err())
				case <-time.After(delay):
					continue
				}
			}
		}
	}

	return fmt.Errorf("SSH connection test failed after %d attempts", maxRetries)
}

// deployWithContext executes deployment with context
func (h *WebhookHandler) deployWithContext(ctx context.Context, repo models.Repository, branch, requestID string) error {
	resultChan := make(chan error, 1)
	progressChan := make(chan string, 10)

	go func() {
		defer close(resultChan)
		defer close(progressChan)

		progressChan <- "Starting deployment"
		err := h.deploymentService.DeployDirect(repo, branch)
		resultChan <- err
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Error("[%s] Deployment timeout exceeded", requestID)
			return fmt.Errorf("deployment timeout: %v", ctx.Err())

		case progress := <-progressChan:
			if progress != "" {
				h.logger.Info("[%s] Deployment progress: %s", requestID, progress)
			}

		case <-ticker.C:
			h.logger.Info("[%s] Deployment still in progress...", requestID)

		case err := <-resultChan:
			return err
		}
	}
}

// applyGitSafetyFixes applies Git safety configurations
func (h *WebhookHandler) applyGitSafetyFixes(repoPath, requestID string) error {
	h.logger.Debug("[%s] Applying Git safety fixes for: %s", requestID, repoPath)

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "unknown"
	}

	cmd := exec.Command("git", "config", "--global", "safe.directory", "*")
	if err := cmd.Run(); err != nil {
		h.logger.Warning("[%s] Failed to set global safe directory: %v", requestID, err)
	}

	paths := []string{
		repoPath,
		filepath.Dir(repoPath),
		h.config.Settings.WorkDir,
	}

	for _, path := range paths {
		cmd = exec.Command("git", "config", "--global", "--add", "safe.directory", path)
		if err := cmd.Run(); err != nil {
			h.logger.Debug("[%s] Failed to add safe directory %s: %v", requestID, path, err)
		}
	}

	if os.Getuid() == 0 {
		h.logger.Debug("[%s] Running as root, fixing ownership", requestID)
		if err := h.fixOwnership(repoPath); err != nil {
			h.logger.Warning("[%s] Failed to fix ownership: %v", requestID, err)
		}
	}

	return nil
}

// fixOwnership fixes directory ownership
func (h *WebhookHandler) fixOwnership(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("chown", "-R", "root:root", path)
	return cmd.Run()
}

// sendResponse sends the JSON response
func (h *WebhookHandler) sendResponse(w http.ResponseWriter, statusCode int, response *WebhookResponse) {
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
		fmt.Fprintf(w, `{"status":"error","message":"Failed to encode response"}`)
	}
}

// Helper functions for webhook
func (h *WebhookHandler) generateRequestID() string {
	return fmt.Sprintf("%d-%s", time.Now().Unix(), h.randomString(6))
}

func (h *WebhookHandler) randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

func (h *WebhookHandler) getShortCommitID(commitID string) string {
	if len(commitID) > 7 {
		return commitID[:7]
	}
	return commitID
}

func (h *WebhookHandler) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (h *WebhookHandler) getPusherInfo(webhook *models.GitHubWebhook) string {
	if webhook.Pusher.Name != "" {
		return webhook.Pusher.Name
	}
	if webhook.Sender.Login != "" {
		return webhook.Sender.Login
	}
	if webhook.HeadCommit.Author.Name != "" {
		return webhook.HeadCommit.Author.Name
	}
	return "unknown"
}
