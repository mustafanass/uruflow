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

package docker

const (
	LabelManaged = "uruflow.managed"
	LabelProject = "uruflow.project"
	LabelRelease = "uruflow.release"
	LabelService = "uruflow.service"
	LabelRole    = "uruflow.role"

	RoleRegistry = "registry"
	RoleService  = "service"
)

func ManagedLabels(project, service, release string) map[string]string {
	labels := map[string]string{
		LabelManaged: "true",
		LabelProject: project,
		LabelRelease: release,
		LabelRole:    RoleService,
	}
	if service != "" {
		labels[LabelService] = service
	}
	return labels
}

func IsManaged(labels map[string]string) bool {
	return labels[LabelManaged] == "true"
}

func ProjectOf(labels map[string]string) string {
	return labels[LabelProject]
}

func ServiceOf(labels map[string]string) string {
	return labels[LabelService]
}
