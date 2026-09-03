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

package pipeline

import (
	"strings"
	"testing"

	"github.com/mustafanass/uruflow/internal/models"
)

func serviceOwnedProject() *models.Project {
	return &models.Project{
		Name: "urufi-prod", Workflow: models.WorkflowBuildDeploy, Builder: "builder-01",
		Runners: []string{"runner-01"}, Services: []models.Service{{
			Name: "core", GitURL: "git@example/core.git", Branch: "main", Dockerfile: "Dockerfile",
		}},
	}
}

func TestProjectValidationAcceptsServiceOwnedSources(t *testing.T) {
	if err := (&Pipeline{}).validateProject(serviceOwnedProject()); err != nil {
		t.Fatalf("service-owned sources rejected: %v", err)
	}
}

func TestProjectValidationRejectsProjectSourceFallback(t *testing.T) {
	project := serviceOwnedProject()
	project.GitURL = "git@example/project.git"
	project.Branch = "main"
	project.Services[0].GitURL = ""
	project.Services[0].Branch = ""

	err := (&Pipeline{}).validateProject(project)
	if err == nil || !strings.Contains(err.Error(), "requires git and branch") {
		t.Fatalf("project source fallback error = %v", err)
	}
}
