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

package registry

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRepositoriesFollowPaginationAndReuseClient(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		username, password, ok := request.BasicAuth()
		if !ok || username != "uruflow" || password != "secret" {
			t.Errorf("basic auth = %q %q %v", username, password, ok)
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("last") == "" {
			writer.Header().Set("Link", `</v2/_catalog?n=200&last=beta>; rel="next"`)
			writer.Write([]byte(`{"repositories":["beta","alpha"]}`))
			return
		}
		writer.Write([]byte(`{"repositories":["gamma","beta"]}`))
	}))
	defer server.Close()

	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	registry := New(Options{
		Address: strings.TrimPrefix(server.URL, "https://"), Username: "uruflow", Password: "secret", CACert: string(certificate),
	}, nil)

	repositories, err := registry.Repositories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(repositories, ",") != "alpha,beta,gamma" || requests != 2 {
		t.Fatalf("repositories=%v requests=%d", repositories, requests)
	}
	first, err := registry.httpClient()
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.httpClient()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("registry HTTP client was rebuilt")
	}
}

func TestCatalogNextRejectsExternalLinks(t *testing.T) {
	if _, err := catalogNext(`<https://example.test/v2/_catalog?last=api>; rel="next"`); err == nil {
		t.Fatal("external catalog link was accepted")
	}
}
