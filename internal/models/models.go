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

package models

// Update: add GitHubWebhook models to handle all github process , add GitLabWebhook models o handle all gitlab process

import "time"

// Config represents the main configuration structure
type Config struct {
	Repositories []Repository  `json:"repositories"`
	Settings     Settings      `json:"settings,omitempty"`
	Webhook      WebhookConfig `json:"webhook,omitempty"`
}

// Repository represents a Git repository configuration
type Repository struct {
	Name         string                       `json:"name"`
	GitURL       string                       `json:"git_url"`
	Branches     []string                     `json:"branches"`
	ComposeFile  string                       `json:"compose_file,omitempty"`
	BranchConfig map[string]BranchEnvironment `json:"branch_config,omitempty"`
	AutoDeploy   bool                         `json:"auto_deploy,omitempty"`
	Enabled      bool                         `json:"enabled,omitempty"`
}

// BranchEnvironment represents branch-specific configuration
type BranchEnvironment struct {
	ProjectName string `json:"project_name,omitempty"`
}

// Settings represents application settings
type Settings struct {
	WorkDir        string `json:"work_dir,omitempty"`
	MaxConcurrent  int    `json:"max_concurrent,omitempty"`
	CleanupEnabled bool   `json:"cleanup_enabled,omitempty"`
	AutoClone      bool   `json:"auto_clone,omitempty"`
}

// WebhookConfig represents webhook server configuration
type WebhookConfig struct {
	Port   string `json:"port,omitempty"`
	Path   string `json:"path,omitempty"`
	Secret string `json:"secret,omitempty"`
}

// DeploymentJob represents a deployment task
type DeploymentJob struct {
	Repository Repository
	Branch     string
	CommitID   string
	CommitMsg  string
	Author     string
}

// GitHubWebhook represents the GitHub webhook payload
type GitHubWebhook struct {
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Repository struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	Sender struct {
		Login     string `json:"login"`
		ID        int    `json:"id"`
		AvatarURL string `json:"avatar_url"`
		Type      string `json:"type"`
	} `json:"sender"`
	Created bool   `json:"created"`
	Deleted bool   `json:"deleted"`
	Forced  bool   `json:"forced"`
	Compare string `json:"compare"`
	Commits []struct {
		ID        string `json:"id"`
		TreeID    string `json:"tree_id"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		URL       string `json:"url"`
		Author    struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username,omitempty"`
		} `json:"author"`
		Committer struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username,omitempty"`
		} `json:"committer"`
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	} `json:"commits"`
	HeadCommit struct {
		ID        string `json:"id"`
		TreeID    string `json:"tree_id"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		URL       string `json:"url"`
		Author    struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username,omitempty"`
		} `json:"author"`
		Committer struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username,omitempty"`
		} `json:"committer"`
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	} `json:"head_commit"`
}

// GitLabWebhook represents the GitLab webhook payload
type GitLabWebhook struct {
	ObjectKind  string `json:"object_kind"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Ref         string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
	UserID      int    `json:"user_id"`
	UserName    string `json:"user_name"`
	UserEmail   string `json:"user_email"`
	UserAvatar  string `json:"user_avatar"`
	Project     struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Description       string `json:"description"`
		WebURL            string `json:"web_url"`
		AvatarURL         string `json:"avatar_url"`
		GitSSHURL         string `json:"git_ssh_url"`
		GitHTTPURL        string `json:"git_http_url"`
		Namespace         string `json:"namespace"`
		PathWithNamespace string `json:"path_with_namespace"`
		DefaultBranch     string `json:"default_branch"`
	} `json:"project"`
	Repository struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description"`
		Homepage    string `json:"homepage"`
	} `json:"repository"`
	Commits []struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		URL       string `json:"url"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
	TotalCommitsCount int `json:"total_commits_count"`
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus struct {
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	Status     string    `json:"status"`
	CommitID   string    `json:"commit_id"`
	CommitMsg  string    `json:"commit_message"`
	Author     string    `json:"author"`
	StartTime  time.Time `json:"start_time"`
	Duration   string    `json:"duration"`
	Error      string    `json:"error,omitempty"`
	Services   []string  `json:"services,omitempty"`
}

// HealthStatus represents the health check response
type HealthStatus struct {
	Status     string    `json:"status"`
	Time       time.Time `json:"time"`
	ActiveJobs int       `json:"active_jobs"`
	QueueSize  int       `json:"queue_size"`
}
