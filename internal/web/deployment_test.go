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

package web

import (
	"testing"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/pipeline"
)

type deploymentOperator struct {
	failure error
	fixtureOperator
	rebuild  bool
	rollback bool
	source   string
	project  string
}

func (f *deploymentOperator) Deploy(project, source string, rollback, rebuild bool) (*models.Release, error) {
	f.calls++
	f.project, f.source, f.rollback, f.rebuild = project, source, rollback, rebuild
	if f.failure != nil {
		return nil, f.failure
	}
	return &models.Release{ID: "new-release", Project: project}, nil
}

func TestRegistryCheckFailureReturnsServiceUnavailable(t *testing.T) {
	operator := &deploymentOperator{failure: pipeline.ErrRegistryUnavailable}
	handler, _ := newConsole(t, operator, true)
	user, csrf := signIn(t, handler, "operator")
	response := request(handler, "POST", "/api/projects/assigned-project/deploy", `{}`, user, csrf)
	if response.Code != 503 {
		t.Fatalf("registry failure status: %d %s", response.Code, response.Body)
	}
}

func TestDeploymentRebuildChoiceReachesPipelineWithExistingPermissions(t *testing.T) {
	operator := &deploymentOperator{}
	handler, _ := newConsole(t, operator, true)
	user, csrf := signIn(t, handler, "operator")
	for _, body := range []string{`{}`, `{"rebuild":true}`} {
		response := request(handler, "POST", "/api/projects/assigned-project/deploy", body, user, csrf)
		if response.Code != 200 || operator.rebuild != (body != `{}`) || operator.project != "assigned-project" || operator.rollback {
			t.Fatalf("deployment choice lost: %d %s %+v", response.Code, response.Body, operator)
		}
	}
	calls := operator.calls
	if response := request(handler, "POST", "/api/projects/private-project/deploy", `{"rebuild":true}`, user, csrf); response.Code != 404 {
		t.Fatalf("project grants bypassed: %d", response.Code)
	}
	viewer, viewerCSRF := signIn(t, handler, "viewer")
	if response := request(handler, "POST", "/api/projects/assigned-project/deploy", `{"rebuild":true}`, viewer, viewerCSRF); response.Code != 403 {
		t.Fatalf("viewer started rebuild: %d", response.Code)
	}
	if operator.calls != calls {
		t.Fatal("unauthorized deployment reached pipeline")
	}
}
