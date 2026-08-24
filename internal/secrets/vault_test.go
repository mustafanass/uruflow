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

package secrets

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSealAndOpenRoundTrip(t *testing.T) {
	vault, err := LoadOrCreateVault(filepath.Join(t.TempDir(), "secrets.key"))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}

	for _, plaintext := range []string{"", "short", "postgres://user:p@ss@host:5432/db?x=1",
		strings.Repeat("long", 5000), "unicode ✔ ünïcode"} {
		sealed, err := vault.Seal(plaintext)
		if err != nil {
			t.Fatalf("seal: %v", err)
		}
		if bytes.Contains(sealed, []byte(plaintext)) && plaintext != "" {
			t.Fatalf("ciphertext contains the plaintext: %q", plaintext)
		}

		opened, err := vault.Open(sealed)
		if err != nil || opened != plaintext {
			t.Fatalf("open = %q err = %v, want %q", opened, err, plaintext)
		}
	}
}

func TestSealIsNotDeterministic(t *testing.T) {
	vault, _ := LoadOrCreateVault(filepath.Join(t.TempDir(), "secrets.key"))

	first, _ := vault.Seal("same value")
	second, _ := vault.Seal("same value")

	if bytes.Equal(first, second) {
		t.Fatal("sealing the same value twice produced identical ciphertext")
	}
}

func TestOpenRejectsTamperedCiphertext(t *testing.T) {
	vault, _ := LoadOrCreateVault(filepath.Join(t.TempDir(), "secrets.key"))
	sealed, _ := vault.Seal("original")

	tampered := append([]byte{}, sealed...)
	tampered[len(tampered)-1] ^= 0xFF

	if _, err := vault.Open(tampered); err == nil {
		t.Fatal("a tampered ciphertext was accepted")
	}
	if _, err := vault.Open(sealed[:3]); err == nil {
		t.Fatal("a truncated ciphertext was accepted")
	}
}

func TestAnotherKeyCannotOpen(t *testing.T) {
	first, _ := LoadOrCreateVault(filepath.Join(t.TempDir(), "a.key"))
	second, _ := LoadOrCreateVault(filepath.Join(t.TempDir(), "b.key"))

	sealed, _ := first.Seal("classified")
	if _, err := second.Open(sealed); err == nil {
		t.Fatal("a different key opened the ciphertext")
	}
}

func TestKeyIsPersistedAndReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.key")

	first, _ := LoadOrCreateVault(path)
	sealed, _ := first.Seal("persisted")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("key file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode is %v, want 0600", info.Mode().Perm())
	}

	second, err := LoadOrCreateVault(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if opened, err := second.Open(sealed); err != nil || opened != "persisted" {
		t.Fatalf("reopened vault could not decrypt: %q %v", opened, err)
	}
}

func TestReferenceParsing(t *testing.T) {
	if got := References("${secret:db_url}"); len(got) != 1 || got[0] != "db_url" {
		t.Fatalf("references = %v", got)
	}
	if got := References("prefix ${secret:a} middle ${secret:b} suffix"); len(got) != 2 {
		t.Fatalf("references = %v", got)
	}
	if References("${secret:}") != nil && len(References("${secret:}")) != 0 {
		t.Error("an empty name was accepted")
	}
}

func TestResolveSubstitutesAndReportsMissing(t *testing.T) {
	lookup := func(name string) (string, error) {
		if name == "db_url" {
			return "postgres://real", nil
		}
		return "", os.ErrNotExist
	}

	env := map[string]string{
		"DATABASE_URL": "${secret:db_url}",
		"COMBINED":     "prefix-${secret:db_url}-suffix",
		"PLAIN":        "unchanged",
	}

	resolved, err := Resolve(env, lookup)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved["DATABASE_URL"] != "postgres://real" {
		t.Errorf("DATABASE_URL = %q", resolved["DATABASE_URL"])
	}
	if resolved["COMBINED"] != "prefix-postgres://real-suffix" {
		t.Errorf("COMBINED = %q", resolved["COMBINED"])
	}
	if resolved["PLAIN"] != "unchanged" {
		t.Errorf("PLAIN = %q", resolved["PLAIN"])
	}

	missing := map[string]string{"X": "${secret:absent}"}
	if _, err := Resolve(missing, lookup); err == nil {
		t.Fatal("a missing secret was not reported")
	} else if !strings.Contains(err.Error(), "X") {
		t.Errorf("the error does not name the variable: %v", err)
	}
}

func TestNamesCollectsEveryReference(t *testing.T) {
	env := map[string]string{
		"A": "${secret:one}",
		"B": "${secret:two} and ${secret:one}",
		"C": "plain",
	}

	names := Names(env)
	if len(names) != 2 || names[0] != "one" || names[1] != "two" {
		t.Fatalf("names = %v", names)
	}
}
