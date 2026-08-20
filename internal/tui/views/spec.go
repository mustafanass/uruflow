/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 *
 * uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * MIT License for more details.
 *
 * You should have received a copy of the MIT License
 * along with uruflow. If not, see the LICENSE file in the project root.
 */

package views

import (
	"strings"

	"github.com/urustack/uruflow/internal/models"
)

func parsePorts(value string) ([]models.Port, error) {
	return models.ParsePorts(fields(value))
}

func formatPorts(ports []models.Port) string {
	return strings.Join(models.FormatPorts(ports), ",")
}

func parseVolumes(value string) ([]models.Volume, error) {
	return models.ParseVolumes(fields(value))
}

func formatVolumes(volumes []models.Volume) string {
	return strings.Join(models.FormatVolumes(volumes), ",")
}

func fields(value string) []string {
	parts := make([]string, 0)
	for _, entry := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func formatBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
