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

package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnreferencedKeepsSharedSecrets(t *testing.T) {
	got := unreferenced([]string{"removed", "shared"}, map[string]bool{"shared": true})
	if len(got) != 1 || got[0] != "removed" {
		t.Fatalf("unreferenced = %v", got)
	}
}

func TestWritePrivateFileIsAtomicAndRemovable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prod.env")
	if err := writePrivateFile(path, "MODE=prod\n"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode())
	}
	if err := writePrivateFile(path, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("empty environment did not remove file: %v", err)
	}
}
