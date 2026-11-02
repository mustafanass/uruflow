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

package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"golang.org/x/crypto/bcrypt"
)

func TestServerWithoutArgumentsPrintsHelp(t *testing.T) {
	command := newServerCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "serve") || !strings.Contains(output.String(), "admin") {
		t.Fatal(output.String())
	}
}

func TestLocalAdminRecovery(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.DataDir = dir
	path := filepath.Join(dir, "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.New(cfg.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, _ := bcrypt.GenerateFromPassword([]byte("old password for testing"), bcrypt.MinCost)
	for _, role := range []string{"admin", "viewer"} {
		if err = store.CreateConsoleUser(&storage.ConsoleUser{Username: role, Role: role, PasswordHash: hash}); err != nil {
			t.Fatal(err)
		}
	}
	previous := serverConfigPath
	serverConfigPath = path
	defer func() { serverConfigPath = previous }()
	for _, role := range []string{"viewer", "admin"} {
		command := adminCommand()
		command.SetArgs([]string{"reset-password", role, "--password-stdin"})
		command.SetIn(strings.NewReader("new recovery password for testing\n"))
		command.SetOut(io.Discard)
		err = command.Execute()
		if role == "viewer" && err == nil {
			t.Fatal("viewer recovery was allowed")
		}
		if role == "admin" && err != nil {
			t.Fatal(err)
		}
	}
	user, err := store.GetConsoleUser("admin")
	if err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword(user.PasswordHash, []byte("new recovery password for testing")) != nil {
		t.Fatal("password not reset")
	}
	audit, err := store.ListAudit(10)
	if err != nil || len(audit) != 1 || audit[0].Action != "user.password.recover" {
		t.Fatal(audit, err)
	}
}
