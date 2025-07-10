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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"uruflow.com/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "⚙️ Configuration management commands",
	Long:  `Manage UruFlow configuration: show info, reload, validate.`,
}

var configInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "📋 Show configuration information",
	Long:  `Display current configuration details and file location.`,
	Run:   showConfigInfo,
}

var configReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "🔄 Reload configuration",
	Long:  `Reload configuration from file without restarting the application.`,
	Run:   reloadConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInfoCmd)
	configCmd.AddCommand(configReloadCmd)
}

func showConfigInfo(cmd *cobra.Command, args []string) {
	configPath := config.GetConfigPath(envManager)

	fmt.Printf("📋 Configuration Information\n")
	fmt.Printf("============================\n\n")

	fmt.Printf("📄 Config file: %s\n", configPath)
	if fileExists(configPath) {
		fmt.Printf("	✅ File exists\n")
	} else {
		fmt.Printf("	❌ File does not exist\n")
	}
	fmt.Printf("📁 Config dir env: %s\n", envManager.ConfigDir)
	fmt.Printf("📝 Log dir env: %s\n", envManager.LogDir)
	fmt.Printf("\n")

	fmt.Printf("⚙️ Settings:\n")
	fmt.Printf("   📂 Work directory: %s\n", cfg.Settings.WorkDir)
	fmt.Printf("   🔢 Max concurrent: %d\n", cfg.Settings.MaxConcurrent)
	fmt.Printf("   🔄 Auto clone: %t\n", cfg.Settings.AutoClone)
	fmt.Printf("   🧹 Cleanup enabled: %t\n", cfg.Settings.CleanupEnabled)
	fmt.Printf("\n")

	fmt.Printf("🌐 Webhook:\n")
	fmt.Printf("   🔌 Port: %s\n", cfg.Webhook.Port)
	fmt.Printf("   📍 Path: %s\n", cfg.Webhook.Path)
	fmt.Printf("   🔐 Secret: %s\n", getSecretDisplay(cfg.Webhook.Secret))
	fmt.Printf("\n")

	enabledCount := 0
	for _, repo := range cfg.Repositories {
		if repo.Enabled {
			enabledCount++
		}
	}

	fmt.Printf("📦 Repositories:\n")
	fmt.Printf("   📊 Total: %d\n", len(cfg.Repositories))
	fmt.Printf("   ✅ Enabled: %d\n", enabledCount)
	fmt.Printf("   ❌ Disabled: %d\n", len(cfg.Repositories)-enabledCount)
}

func reloadConfig(cmd *cobra.Command, args []string) {
	configPath := filepath.Join(envManager.ConfigDir, "config.json")
	logger.Config("Reloading configuration from: %s", configPath)

	newConfig, err := config.Load(envManager)
	if err != nil {
		logger.Error("Failed to reload configuration: %v", err)
		fmt.Printf("❌ Failed to reload configuration: %v\n", err)
		return
	}

	repositoryService.UpdateConfig(newConfig)
	cfg = newConfig

	logger.Success("Configuration reloaded successfully")
	fmt.Printf("✅ Configuration reloaded successfully\n")
	fmt.Printf("📦 Managing %d repositories\n", len(cfg.Repositories))
}

func getSecretDisplay(secret string) string {
	if secret == "" {
		return "(not set)"
	}
	if len(secret) <= 4 {
		return "***"
	}
	return secret[:2] + "***" + secret[len(secret)-2:]
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
