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

package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDoesNotRegenerateMissingCredentials(t *testing.T) {
	cfg := Default()
	cfg.Server.Advertise = "127.0.0.1"
	cfg.Registry.Host = "127.0.0.1"
	cfg.Registry.Password = ""
	cfg.Webhook.Secret = ""
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Registry.Password != "" || loaded.Webhook.Secret != "" {
		t.Fatalf("missing credentials were regenerated: password=%q secret=%q", loaded.Registry.Password, loaded.Webhook.Secret)
	}
	if err := loaded.Validate(); err == nil || !strings.Contains(err.Error(), "registry.password") {
		t.Fatalf("validation error = %v", err)
	}
	loaded.Registry.Password = strings.Repeat("p", 32)
	if err := loaded.Validate(); err == nil || !strings.Contains(err.Error(), "webhook.secret") {
		t.Fatalf("validation error = %v", err)
	}
}

func TestWebInterfaceIsPrivateUnlessTLSExplicitlyEnabled(t *testing.T) {
	cfg := Default()
	cfg.Server.Advertise = "control.test"
	cfg.Registry.Host = "control.test"
	if cfg.WebAddr() != "127.0.0.1:9002" {
		t.Fatalf("unsafe default: %s", cfg.WebAddr())
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Web.Host = "0.0.0.0"
	if err := cfg.Validate(); err == nil {
		t.Fatal("plaintext public listener accepted")
	}
	cfg.Web.TLS = true
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Web.Port = cfg.Server.HTTPPort
	if err := cfg.Validate(); err == nil {
		t.Fatal("webhook and operator ports overlap")
	}
}

func TestWebPublicOriginRequiresHTTPS(t *testing.T) {
	cfg := Default()
	cfg.Server.Advertise = "localhost"
	cfg.Registry.Host = "localhost"
	cfg.Web.PublicURL = "http://console.example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("plaintext external public URL accepted")
	}
	cfg.Web.PublicURL = "https://console.example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Web.PublicURL = "https://console.example.com/subpath"
	if err := cfg.Validate(); err == nil {
		t.Fatal("non-origin public URL accepted")
	}
}
