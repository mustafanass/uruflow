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

package pki

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPartialCAMaterialFailsClosed(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(certPath, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateCA(certPath, keyPath); err == nil {
		t.Fatal("partial CA material was replaced")
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("a new CA key was created: %v", err)
	}
}

func TestMismatchedCAKeyIsRejected(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	firstCert := filepath.Join(first, "ca.crt")
	firstKey := filepath.Join(first, "ca.key")
	secondKey := filepath.Join(second, "ca.key")
	if _, err := LoadOrCreateCA(firstCert, firstKey); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateCA(filepath.Join(second, "ca.crt"), secondKey); err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(secondKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(firstKey, key, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateCA(firstCert, firstKey); err == nil {
		t.Fatal("mismatched CA material was accepted")
	}
}
