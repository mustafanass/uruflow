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

	"github.com/mustafanass/uruflow/internal/pipeline"
)

type deletionOperator struct {
	failure error
	fixtureOperator
	project string
	volumes bool
	images  bool
}

func (f *deletionOperator) DeleteProject(project string, volumes, images bool) error {
	f.calls++
	f.project, f.volumes, f.images = project, volumes, images
	return f.failure
}

func TestProjectDeletionCarriesTheChosenDataAndIsAudited(t *testing.T) {
	operator := &deletionOperator{}
	handler, store := newConsole(t, operator, true)
	admin, csrf := signIn(t, handler, "admin")
	if response := request(handler, "DELETE", "/api/projects/assigned-project", `{"volumes":true}`, admin, ""); response.Code != 403 || operator.calls != 0 {
		t.Fatalf("project deleted without CSRF: %d", response.Code)
	}
	response := request(handler, "DELETE", "/api/projects/assigned-project", `{"volumes":true}`, admin, csrf)
	if response.Code != 200 || operator.project != "assigned-project" || !operator.volumes || operator.images {
		t.Fatalf("deletion choice lost: %d %s %+v", response.Code, response.Body, operator)
	}
	entries, err := store.ListAudit(10)
	if err != nil || len(entries) < 2 || entries[0].Action != "project.delete" || entries[0].Resource != "assigned-project" || entries[0].Outcome != "succeeded" {
		t.Fatalf("deletion not audited: %+v %v", entries, err)
	}
	operator.failure = pipeline.ErrRegistryBusy
	if response = request(handler, "DELETE", "/api/projects/assigned-project", `{}`, admin, csrf); response.Code != 409 {
		t.Fatalf("busy fleet status: %d %s", response.Code, response.Body)
	}
}
