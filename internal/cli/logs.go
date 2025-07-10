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
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "📝 Show application logs",
	Long:  `Display UruFlow application logs.`,
	Run:   showLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().IntP("tail", "t", 50, "Number of lines to show from the end")
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().StringP("grep", "g", "", "Filter logs containing this text")
	logsCmd.Flags().BoolP("today", "", false, "Show only today's logs")
}

func showLogs(cmd *cobra.Command, args []string) {
	var logDir = envManager.LogDir
	if logDir == "" {
		fmt.Fprintf(os.Stderr, "❌ Environment variable URUFLOW_LOG_DIR is not set\n")
		logger.Warning("Please set environment variable URUFLOW_LOG_DIR")
		return
	}

	tail, _ := cmd.Flags().GetInt("tail")
	follow, _ := cmd.Flags().GetBool("follow")
	grep, _ := cmd.Flags().GetString("grep")
	today, _ := cmd.Flags().GetBool("today")

	var logFile string
	if today {
		logFile = filepath.Join(logDir, fmt.Sprintf("uruflow-%s.log", time.Now().Format("2006-01-02")))
	} else {
		logFile = findMostRecentLogFile(logDir)
		if logFile == "" {
			fmt.Printf("❌ No log files found in: %s\n", logDir)
			return
		}
	}

	if !fileExists(logFile) {
		fmt.Printf("❌ Log file not found: %s\n", logFile)
		return
	}

	fmt.Printf("📄 Showing logs from: %s\n\n", logFile)

	var cmdArgs []string
	if follow {
		cmdArgs = []string{"tail", "-f"}
		if tail > 0 {
			cmdArgs = append(cmdArgs, "-n", fmt.Sprintf("%d", tail))
		}
	} else {
		cmdArgs = []string{"tail", "-n", fmt.Sprintf("%d", tail)}
	}

	if grep != "" {
		cmdArgs = append(cmdArgs, logFile)
		grepCmd := exec.Command("grep", grep)
		tailCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		pipe, err := tailCmd.StdoutPipe()
		if err != nil {
			fmt.Printf("❌ Error creating pipe: %v\n", err)
			return
		}
		grepCmd.Stdin = pipe
		grepCmd.Stdout = os.Stdout
		grepCmd.Stderr = os.Stderr
		if err := tailCmd.Start(); err != nil {
			fmt.Printf("❌ Error starting tail: %v\n", err)
			return
		}
		if err := grepCmd.Run(); err != nil {
			if err.Error() != "exit status 1" {
				fmt.Printf("❌ Error running grep: %v\n", err)
			}
		}
		tailCmd.Wait()
	} else {
		cmdArgs = append(cmdArgs, logFile)
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("❌ Error showing logs: %v\n", err)
			return
		}
	}
}

// Helper function to find the most recent log file
func findMostRecentLogFile(logDir string) string {
	files, err := filepath.Glob(filepath.Join(logDir, "uruflow-*.log"))
	if err != nil || len(files) == 0 {
		return ""
	}

	var mostRecent string
	var mostRecentTime time.Time

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if mostRecent == "" || info.ModTime().After(mostRecentTime) {
			mostRecent = file
			mostRecentTime = info.ModTime()
		}
	}

	return mostRecent
}
