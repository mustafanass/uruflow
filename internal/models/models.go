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
	Repository struct {
		Name     string `json:"name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
	} `json:"repository"`
	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"head_commit"`
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
