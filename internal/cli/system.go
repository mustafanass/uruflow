package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "System diagnostics and checks",
	Long:  `Run system diagnostics to check permissions, users, and Docker access.`,
}

var systemCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check system permissions and setup",
	Long:  `Check current user, permissions, Docker access, and Git configuration.`,
	Run:   runSystemCheck,
}

func init() {
	rootCmd.AddCommand(systemCmd)
	systemCmd.AddCommand(systemCheckCmd)
}

func runSystemCheck(cmd *cobra.Command, args []string) {
	fmt.Printf(" Uruflow System Diagnostics\n")
	fmt.Printf("===============================\n\n")

	checkCurrentUser()
	checkDockerAccess()
	checkGitConfiguration()
	checkWorkDirectoryPermissions()
	checkSSHSetup()
}

func checkCurrentUser() {
	fmt.Printf(" User Information:\n")

	// Current user
	currentUser, err := user.Current()
	if err != nil {
		fmt.Printf("   Could not get current user: %v\n", err)
		return
	}

	fmt.Printf("   Username: %s\n", currentUser.Username)
	fmt.Printf("   UID: %s\n", currentUser.Uid)
	fmt.Printf("   Home: %s\n", currentUser.HomeDir)

	uid, _ := strconv.Atoi(currentUser.Uid)
	if uid == 0 {
		fmt.Printf("   🔴 Running as ROOT user\n")
	} else {
		fmt.Printf("   🟢 Running as non-root user\n")
	}

	cmd := exec.Command("groups")
	if output, err := cmd.Output(); err == nil {
		groups := strings.TrimSpace(string(output))
		fmt.Printf("   Groups: %s\n", groups)

		if strings.Contains(groups, "docker") {
			fmt.Printf("   🟢 User is in docker group\n")
		} else {
			fmt.Printf("   🔴 User is NOT in docker group\n")
		}
	} else {
		fmt.Printf("    Could not check groups: %v\n", err)
	}

	fmt.Printf("\n")
}

func checkDockerAccess() {
	fmt.Printf(" Docker Access:\n")

	// Check if docker command exists
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Printf("    Docker command not found\n")
		fmt.Printf("\n")
		return
	}

	// Test docker access
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	if output, err := cmd.Output(); err != nil {
		fmt.Printf("    Cannot access Docker daemon: %v\n", err)
	} else {
		version := strings.TrimSpace(string(output))
		fmt.Printf("   🟢 Docker access OK (Server: %s)\n", version)
	}

	cmd = exec.Command("docker", "compose", "version", "--short")
	if output, err := cmd.Output(); err != nil {
		fmt.Printf("   🔴 Docker Compose not available: %v\n", err)
	} else {
		version := strings.TrimSpace(string(output))
		fmt.Printf("   🟢 Docker Compose available (%s)\n", version)
	}

	fmt.Printf("\n")
}

// Check git Configurations
func checkGitConfiguration() {
	fmt.Printf(" Git Configuration:\n")
	cmd := exec.Command("git", "config", "--global", "--get-all", "safe.directory")
	if output, err := cmd.Output(); err != nil {
		fmt.Printf("   🔴 No safe directories configured\n")
	} else {
		dirs := strings.Split(strings.TrimSpace(string(output)), "\n")
		fmt.Printf("   🟢 Safe directories configured (%d):\n", len(dirs))
		for _, dir := range dirs {
			if dir != "" {
				fmt.Printf("      - %s\n", dir)
			}
		}
	}
	fmt.Printf("\n")
}

func checkWorkDirectoryPermissions() {
	fmt.Printf("Work Directory Permissions:\n")

	workDir := cfg.Settings.WorkDir
	fmt.Printf("Work directory: %s\n", workDir)

	if info, err := os.Stat(workDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("    Work directory does not exist\n")
		} else {
			fmt.Printf("    Cannot access work directory: %v\n", err)
		}
	} else {
		fmt.Printf("   🟢 Work directory exists\n")
		fmt.Printf("   Permissions: %s\n", info.Mode().String())
	}
	fmt.Printf("\n")
}

func checkSSHSetup() {
	fmt.Printf(" SSH Configuration:\n")

	if gitService.IsSSHAvailable() {
		fmt.Printf("   🟢 SSH service is available\n")

		if err := gitService.TestSSHConnection(); err != nil {
			fmt.Printf("   🔴 SSH connection test failed: %v\n", err)
		} else {
			fmt.Printf("   🟢 SSH connection test passed\n")
		}
	} else {
		fmt.Printf("    SSH service not configured\n")
		fmt.Printf("    Try: uruflow ssh setup\n")
	}
	fmt.Printf("\n")
}
