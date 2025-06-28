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
	"fmt"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH management commands",
	Long:  `Manage SSH configuration for Git operations.`,
}

var sshStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SSH status",
	Long:  `Show the current SSH configuration status.`,
	Run:   showSSHStatus,
}

var sshTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test SSH connection",
	Long:  `Test SSH connection to GitHub.`,
	Run:   testSSH,
}

var sshSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Show SSH setup instructions",
	Long:  `Display instructions for setting up SSH keys.`,
	Run:   showSSHSetup,
}

func init() {
	rootCmd.AddCommand(sshCmd)
	sshCmd.AddCommand(sshStatusCmd)
	sshCmd.AddCommand(sshTestCmd)
	sshCmd.AddCommand(sshSetupCmd)
}

// getSSHDirectory returns the SSH directory path
func getSSHDirectory() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %v", err)
	}
	return filepath.Join(currentUser.HomeDir, ".ssh"), nil
}

// getSSHKeyPaths returns common SSH key file paths
func getSSHKeyPaths() ([]string, error) {
	sshDir, err := getSSHDirectory()
	if err != nil {
		return nil, err
	}

	keyNames := []string{"id_rsa", "id_ed25519", "id_ecdsa", "id_dsa"}
	var keyPaths []string

	for _, keyName := range keyNames {
		keyPaths = append(keyPaths, filepath.Join(sshDir, keyName))
	}

	return keyPaths, nil
}

// findExistingSSHKey returns the first SSH key that exists
func findExistingSSHKey() (string, bool) {
	keyPaths, err := getSSHKeyPaths()
	if err != nil {
		return "", false
	}

	for _, keyPath := range keyPaths {
		if fileExists(keyPath) {
			return keyPath, true
		}
	}

	return "", false
}

// showSSHStatus displays SSH configuration status using clean Go APIs
func showSSHStatus(cmd *cobra.Command, args []string) {
	fmt.Printf("SSH Status\n")
	fmt.Printf("===============\n\n")

	if gitService.IsSSHAvailable() {
		fmt.Printf("SSH is configured and ready\n")

		if err := gitService.TestSSHConnection(); err != nil {
			fmt.Printf("SSH configured but connection test failed: %v\n", err)
		} else {
			fmt.Printf("GitHub connection test passed\n")
		}
	} else {
		fmt.Printf("SSH is not configured\n")
		fmt.Printf("\nTo set up SSH:\n")
		fmt.Printf("uruflow ssh setup\n")
	}

	fmt.Printf("\nSSH Configuration:\n")

	sshDir, err := getSSHDirectory()
	if err != nil {
		fmt.Printf("Cannot determine SSH directory: %v\n", err)
		return
	}

	fmt.Printf("SSH Directory: %s\n", sshDir)

	if fileExists(sshDir) {
		fmt.Printf("SSH Directory:  Exists\n")
	} else {
		fmt.Printf("SSH Directory:  Not found\n")
		return
	}

	if keyPath, found := findExistingSSHKey(); found {
		keyName := filepath.Base(keyPath)
		fmt.Printf("SSH Key (%s):  Found\n", keyName)

		pubKeyPath := keyPath + ".pub"
		if fileExists(pubKeyPath) {
			fmt.Printf("Public Key (%s.pub):  Found\n", keyName)
		} else {
			fmt.Printf("Public Key (%s.pub):  Missing\n", keyName)
		}
	} else {
		fmt.Printf("SSH Keys:  No keys found\n")
	}
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// testSSH tests SSH connection to GitHub
func testSSH(cmd *cobra.Command, args []string) {
	fmt.Printf("Testing SSH connection to GitHub...\n\n")

	if !gitService.IsSSHAvailable() {
		fmt.Printf("SSH is not configured\n")
		fmt.Printf("Run 'uruflow ssh setup' for setup instructions\n")
		return
	}
	logger.Info("Testing SSH connection...")
	if err := gitService.TestSSHConnection(); err != nil {
		logger.Error("SSH connection test failed: %v", err)
		fmt.Printf("SSH connection test failed: %v\n", err)
		return
	}

	logger.Success("SSH connection test passed")
	fmt.Printf("SSH connection to GitHub successful!\n")
}

// showSSHSetup displays platform-aware SSH setup instructions
func showSSHSetup(cmd *cobra.Command, args []string) {
	fmt.Printf("SSH Setup Instructions\n")
	fmt.Printf("=========================\n\n")

	// Get SSH directory for platform-specific paths
	sshDir, err := getSSHDirectory()
	if err != nil {
		fmt.Printf("Cannot determine SSH directory: %v\n", err)
		return
	}

	fmt.Printf("Generate SSH Key:\n")
	if runtime.GOOS == "windows" {
		fmt.Printf("ssh-keygen -t ed25519 -C \"your-email@example.com\" -f \"%s\\id_ed25519\"\n", sshDir)
	} else {
		fmt.Printf("ssh-keygen -t ed25519 -C \"your-email@example.com\" -f \"%s/id_ed25519\"\n", sshDir)
	}
	fmt.Printf("(Press Enter for no passphrase, or set one if you prefer)\n\n")

	fmt.Printf("2️ Copy Public Key:\n")
	pubKeyPath := filepath.Join(sshDir, "id_ed25519.pub")
	if runtime.GOOS == "windows" {
		fmt.Printf("   type \"%s\"\n", pubKeyPath)
	} else {
		fmt.Printf("   cat \"%s\"\n", pubKeyPath)
	}
	fmt.Printf("   (Copy the entire output)\n\n")

	fmt.Printf("3  Add to GitHub:\n")
	fmt.Printf("   • Go to: https://github.com/settings/ssh/new\n")
	fmt.Printf("   • Paste your public key\n")
	fmt.Printf("   • Give it a title (e.g., 'UruFlow Server')\n")
	fmt.Printf("   • Click 'Add SSH key'\n\n")

	fmt.Printf("4️  Test Connection:\n")
	fmt.Printf("   uruflow ssh test\n\n")

	fmt.Printf("Current Status:\n")

	// Check current SSH status
	if fileExists(sshDir) {
		fmt.Printf("   SSH Directory: %s\n", sshDir)

		if keyPath, found := findExistingSSHKey(); found {
			keyName := filepath.Base(keyPath)
			fmt.Printf("   Existing Key: %s\n", keyName)

			pubKeyPath := keyPath + ".pub"
			if fileExists(pubKeyPath) {
				fmt.Printf("   Public Key: %s\n", pubKeyPath)
				fmt.Printf("   \n You can copy your public key with:\n")
				if runtime.GOOS == "windows" {
					fmt.Printf("      type \"%s\"\n", pubKeyPath)
				} else {
					fmt.Printf("      cat \"%s\"\n", pubKeyPath)
				}
			} else {
				fmt.Printf("   Public Key:  Missing %s\n", pubKeyPath)
			}
		} else {
			fmt.Printf("   SSH Keys:  No keys found in %s\n", sshDir)
		}
	} else {
		fmt.Printf("   SSH Directory:  %s (will be created)\n", sshDir)
	}
}
