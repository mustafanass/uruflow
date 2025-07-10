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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"uruflow.com/internal/utils"
)

// SSHService handles SSH key management for Git operations
type SSHService struct {
	logger     *utils.Logger
	sshKeyPath string
	sshAgent   bool
}

// NewSSHService creates a new SSH service
func NewSSHService(logger *utils.Logger) *SSHService {
	return &SSHService{
		logger:     logger,
		sshKeyPath: "",
		sshAgent:   false,
	}
}

// SetupSSHKey configures SSH key for Git operations
func (s *SSHService) SetupSSHKey(keyPath string) error {
	if keyPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %v", err)
		}

		defaultKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
		if _, err := os.Stat(defaultKeyPath); err == nil {
			keyPath = defaultKeyPath
		} else {
			return fmt.Errorf("no SSH key specified and default key not found at %s", defaultKeyPath)
		}
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH key not found: %s", keyPath)
	}

	if err := s.checkKeyPermissions(keyPath); err != nil {
		return fmt.Errorf("SSH key permission error: %v", err)
	}

	s.sshKeyPath = keyPath
	s.logger.Security("SSH key configured: %s", keyPath)

	if err := s.testGitHubConnection(); err != nil {
		return fmt.Errorf("GitHub SSH connection test failed: %v", err)
	}

	s.logger.Success("SSH authentication configured successfully")
	return nil
}

// checkKeyPermissions verifies SSH key has correct permissions
func (s *SSHService) checkKeyPermissions(keyPath string) error {
	fileInfo, err := os.Stat(keyPath)
	if err != nil {
		return err
	}

	mode := fileInfo.Mode()
	if mode.Perm() != 0600 {
		s.logger.Warning("SSH key permissions are %o, should be 600. Attempting to fix...", mode.Perm())
		if err := os.Chmod(keyPath, 0600); err != nil {
			return fmt.Errorf("failed to fix key permissions: %v", err)
		}
		s.logger.Success("Fixed SSH key permissions")
	}

	return nil
}

// testGitHubConnection tests SSH connection to GitHub
func (s *SSHService) testGitHubConnection() error {
	cmd := exec.Command("ssh", "-T", "-o", "StrictHostKeyChecking=no", "git@github.com")
	output, err := cmd.CombinedOutput()

	if err != nil {
		if strings.Contains(string(output), "successfully authenticated") {
			s.logger.Success("GitHub SSH connection test passed")
			return nil
		}
		return fmt.Errorf("GitHub SSH test failed: %v, output: %s", err, output)
	}

	return nil
}

// GetSSHCommand returns SSH command with proper key configuration
func (s *SSHService) GetSSHCommand() []string {
	if s.sshKeyPath == "" {
		return []string{"ssh"}
	}

	return []string{
		"ssh",
		"-i", s.sshKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
}

// SetupGitSSH configures Git to use SSH for operations
func (s *SSHService) SetupGitSSH() error {
	if s.sshKeyPath == "" {
		return fmt.Errorf("SSH key not configured")
	}

	sshWrapper := fmt.Sprintf(`#!/bin/bash
ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$@"
`, s.sshKeyPath)

	wrapperPath := "/tmp/git-ssh-wrapper"
	if err := os.WriteFile(wrapperPath, []byte(sshWrapper), 0755); err != nil {
		return fmt.Errorf("failed to create SSH wrapper: %v", err)
	}

	os.Setenv("GIT_SSH", wrapperPath)
	s.logger.Success("Git SSH configuration completed")

	return nil
}

// GenerateSSHKey creates a new SSH key pair for GitHub access
func (s *SSHService) GenerateSSHKey(keyPath, email string) error {
	if keyPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %v", err)
		}
		keyPath = filepath.Join(homeDir, ".ssh", "id_rsa_uruflow")
	}

	sshDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create SSH directory: %v", err)
	}

	cmd := exec.Command("ssh-keygen",
		"-t", "rsa",
		"-b", "4096",
		"-C", email,
		"-f", keyPath,
		"-N", "",
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to generate SSH key: %v, output: %s", err, output)
	}

	s.logger.Success("SSH key generated: %s", keyPath)
	s.logger.Info("Public key location: %s.pub", keyPath)

	pubKeyContent, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		s.logger.Warning("Failed to read public key: %v", err)
	} else {
		s.logger.Info("Public key content:\n%s", string(pubKeyContent))
		s.logger.Info("Add this public key to your GitHub account's SSH keys")
	}

	return nil
}

// ValidateSSHSetup checks if SSH is properly configured
func (s *SSHService) ValidateSSHSetup() error {
	if s.sshKeyPath == "" {
		return fmt.Errorf("SSH key not configured")
	}

	if _, err := os.Stat(s.sshKeyPath); err != nil {
		return fmt.Errorf("SSH key not found: %s", s.sshKeyPath)
	}

	if err := s.testGitHubConnection(); err != nil {
		return fmt.Errorf("GitHub connection failed: %v", err)
	}

	s.logger.Success("SSH setup validation passed")
	return nil
}

// GetPublicKey returns the public key content
func (s *SSHService) GetPublicKey() (string, error) {
	if s.sshKeyPath == "" {
		return "", fmt.Errorf("SSH key not configured")
	}

	pubKeyPath := s.sshKeyPath + ".pub"
	content, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key: %v", err)
	}

	return string(content), nil
}
