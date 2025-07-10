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

package services

import (
	"encoding/json"
	"os/exec"

	"uruflow.com/internal/models"
	"uruflow.com/internal/utils"
)

// NotificationService handles external notifications
type NotificationService struct {
	webhookURL string
	logger     *utils.Logger
}

// NewNotificationService creates a new notification service
func NewNotificationService(webhookURL string, logger *utils.Logger) *NotificationService {
	return &NotificationService{
		webhookURL: webhookURL,
		logger:     logger,
	}
}

// SendDeploymentStatus sends deployment status to external webhook
func (n *NotificationService) SendDeploymentStatus(status models.DeploymentStatus) {
	if n.webhookURL == "" {
		return
	}
	go func() {
		jsonData, err := json.Marshal(status)
		if err != nil {
			n.logger.Error("Failed to marshal deployment status: %v", err)
			return
		}
		cmd := exec.Command("curl", "-X", "POST",
			"-H", "Content-Type: application/json",
			"-d", string(jsonData),
			n.webhookURL)
		if err := cmd.Run(); err != nil {
			n.logger.Error("Failed to send notification: %v", err)
		} else {
			n.logger.Info("Notification sent for %s:%s", status.Repository, status.Branch)
		}
	}()
}
