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

package env_manager

import "os"

type EnvManager struct {
	ConfigDir string
	LogDir    string
}

func NewEnvManager() *EnvManager {
	logDir, ok := os.LookupEnv("URUFLOW_LOG_DIR")
	if !ok {
		panic("Environment variable 'URUFLOW_LOG_DIR' is not set")
	}
	configDir, ok := os.LookupEnv("URUFLOW_CONFIG_DIR")
	if !ok {
		panic("Environment variable 'URUFLOW_CONFIG_DIR' is not set")
	}
	return &EnvManager{ConfigDir: configDir, LogDir: logDir}
}
