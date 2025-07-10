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

package helper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"uruflow.com/internal/utils"
)

// SSHHelper provides simple SSH key management (one ssh key per service)
type SSHHelper struct {
	logger  *utils.Logger
	keyPath string
	ready   bool
}

// NewSSHHelper creates SSH helper instance
func NewSSHHelper(logger *utils.Logger) *SSHHelper {
	return &SSHHelper{
		logger: logger,
		ready:  false,
	}
}

// EnsureSSHKey checks and sets up SSH key
func (s *SSHHelper) EnsureSSHKey() error {
	keyPath, err := s.findSSHKey()
	if err != nil {
		s.logger.Warning("No SSH key found: %v", err)
		s.showSetupInstructions()
		return nil
	}

	s.keyPath = keyPath
	s.fixPermissions()
	if err := s.testConnection(); err != nil {
		s.logger.Warning("SSH test failed: %v", err)
	} else {
		s.logger.Success("SSH connection OK")
	}
	s.ready = true
	return nil
}

// findSSHKey finds the default SSH key with root user support
func (s *SSHHelper) findSSHKey() (string, error) {
	possiblePaths := []string{}

	if homeDir, err := os.UserHomeDir(); err == nil {
		possiblePaths = append(possiblePaths,
			filepath.Join(homeDir, ".ssh", "id_rsa"),
		)
	}

	if os.Getuid() == 0 {
		possiblePaths = append(possiblePaths,
			"/root/.ssh/id_rsa",
		)
	}

	possiblePaths = append(possiblePaths,
		".ssh/id_rsa",
	)
	for _, keyPath := range possiblePaths {
		if _, err := os.Stat(keyPath); err == nil {
			s.logger.Info("Found SSH key at: %s", keyPath)
			return keyPath, nil
		}
	}
	return "", fmt.Errorf("SSH key not found. Tried: %v", possiblePaths)
}

// fixPermissions create correct SSH key permissions
func (s *SSHHelper) fixPermissions() {
	if s.keyPath == "" {
		return
	}
	os.Chmod(s.keyPath, 0600)
	os.Chmod(s.keyPath+".pub", 0644)
	os.Chmod(filepath.Dir(s.keyPath), 0700)
}

// testConnection tests SSH connection to GitHub with comprehensive error checking
func (s *SSHHelper) testConnection() error {
	if s.keyPath == "" {
		return fmt.Errorf("no SSH key configured")
	}

	if _, err := os.Stat(s.keyPath); err != nil {
		return fmt.Errorf("SSH key file not accessible: %v", err)
	}

	cmd := exec.Command("ssh", "-T", "-i", s.keyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=15",
		"-o", "BatchMode=yes",
		"git@github.com")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if strings.Contains(outputStr, "successfully authenticated") {
		return nil
	}

	if strings.Contains(outputStr, "Permission denied") {
		s.logger.Warning("SSH test output: %s", outputStr)
	}

	if strings.Contains(outputStr, "Permission denied (publickey)") {
		return fmt.Errorf("permission denied - SSH key not authorized on GitHub")
	}

	if strings.Contains(outputStr, "Host key verification failed") {
		return fmt.Errorf("host key verification failed")
	}

	return fmt.Errorf("connection failed: exit status %v, output: %s", err, outputStr)
}

// GetGitEnvironment returns Git environment with SSH setup
func (s *SSHHelper) GetGitEnvironment() []string {
	env := os.Environ()
	if s.ready && s.keyPath != "" {
		absKeyPath, err := filepath.Abs(s.keyPath)
		if err != nil {
			s.logger.Warning("Failed to get absolute path for SSH key: %v", err)
			absKeyPath = s.keyPath
		}

		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o BatchMode=yes -o ConnectTimeout=15", absKeyPath)
		env = append(env, "GIT_SSH_COMMAND="+sshCmd)
		s.logger.Info("Git SSH environment configured with key: %s", absKeyPath)
	} else {
		s.logger.Warning("SSH not ready, Git operations may fail")
	}
	return env
}

// IsReady returns true if SSH is configured
func (s *SSHHelper) IsReady() bool {
	return s.ready
}

// TestGitHubConnection tests the current SSH setup
func (s *SSHHelper) TestGitHubConnection() error {
	if !s.ready {
		return fmt.Errorf("SSH not configured")
	}
	return s.testConnection()
}

// showSetupInstructions displays setup instructions (only simple and nicer logger)
func (s *SSHHelper) showSetupInstructions() {
	s.logger.Info("To setup SSH:")
	s.logger.Info("1. ssh-keygen -t rsa -b 4096 -C \"your-email@example.com\"")
	s.logger.Info("2. cat ~/.ssh/id_rsa.pub")
	s.logger.Info("3. Add the key to GitHub: https://github.com/settings/ssh/new")
}
