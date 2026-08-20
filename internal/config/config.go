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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urustack/uruflow/pkg/helper"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath = "/etc/uruflow/config.yaml"
	DefaultDataDir    = "/var/lib/uruflow"
	DefaultUFPPort    = 9001
	DefaultHTTPPort   = 9000
	DefaultRegistry   = 5000
	DefaultNamespace  = "uruflow"
	DefaultRegistryID = "uruflow-registry"
	RegistryImage     = "registry:2"
	DatabaseFile      = "uruflow.db"
	LogFile           = "uruflow.log"
)

type Config struct {
	path string

	Server   ServerConfig   `yaml:"server"`
	Registry RegistryConfig `yaml:"registry"`
	Webhook  WebhookConfig  `yaml:"webhook"`
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	UFPPort   int    `yaml:"ufp_port"`
	HTTPPort  int    `yaml:"http_port"`
	DataDir   string `yaml:"data_dir"`
	Advertise string `yaml:"advertise"`
}

type RegistryConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Namespace string `yaml:"namespace"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Image     string `yaml:"image"`
	Socket    string `yaml:"socket"`
}

type WebhookConfig struct {
	Path   string `yaml:"path"`
	Secret string `yaml:"secret"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:     "0.0.0.0",
			UFPPort:  DefaultUFPPort,
			HTTPPort: DefaultHTTPPort,
			DataDir:  DefaultDataDir,
		},
		Registry: RegistryConfig{
			Port:      DefaultRegistry,
			Namespace: DefaultNamespace,
			Username:  DefaultNamespace,
			Password:  helper.GenerateToken(),
			Image:     RegistryImage,
			Socket:    "/var/run/docker.sock",
		},
		Webhook: WebhookConfig{
			Path:   "/webhook",
			Secret: helper.GenerateSecret(),
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.normalize()
	cfg.path = path
	return cfg, nil
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	c.path = path
	if err := os.MkdirAll(c.ProjectsDir(), 0o755); err != nil {
		return fmt.Errorf("create projects dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

func (c *Config) Validate() error {
	if c.Registry.Host == "" {
		return fmt.Errorf("registry.host is required: agents must be able to reach the registry by this name")
	}
	if c.Server.Advertise == "" {
		return fmt.Errorf("server.advertise is required: agents dial the server by this name")
	}
	return nil
}

func (c *Config) EnsureDirs() error {
	for _, dir := range []string{c.Server.DataDir, c.PKIDir(), c.RegistryDataDir()} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

func (c *Config) UFPAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.UFPPort)
}

func (c *Config) HTTPAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.HTTPPort)
}

func (c *Config) DatabasePath() string {
	return filepath.Join(c.Server.DataDir, DatabaseFile)
}

func (c *Config) ConfigDir() string {
	if c.path == "" {
		return filepath.Dir(DefaultConfigPath)
	}
	return filepath.Dir(c.path)
}

func (c *Config) ProjectsDir() string {
	return filepath.Join(c.ConfigDir(), "projects")
}

func (c *Config) LogPath() string {
	return filepath.Join(c.Server.DataDir, LogFile)
}

func (c *Config) PKIDir() string {
	return filepath.Join(c.Server.DataDir, "pki")
}

func (c *Config) RegistryDataDir() string {
	return filepath.Join(c.Server.DataDir, "registry")
}

func (c *Config) CACertPath() string       { return filepath.Join(c.PKIDir(), "ca.crt") }
func (c *Config) CAKeyPath() string        { return filepath.Join(c.PKIDir(), "ca.key") }
func (c *Config) ServerCertPath() string   { return filepath.Join(c.PKIDir(), "server.crt") }
func (c *Config) ServerKeyPath() string    { return filepath.Join(c.PKIDir(), "server.key") }
func (c *Config) RegistryCertPath() string { return filepath.Join(c.PKIDir(), "registry.crt") }
func (c *Config) RegistryKeyPath() string  { return filepath.Join(c.PKIDir(), "registry.key") }
func (c *Config) HtpasswdPath() string     { return filepath.Join(c.PKIDir(), "htpasswd") }
func (c *Config) SecretKeyPath() string    { return filepath.Join(c.PKIDir(), "secrets.key") }

func (c *RegistryConfig) Address() string {
	if strings.Contains(c.Host, ":") {
		return c.Host
	}
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *RegistryConfig) Hostname() string {
	if host, _, found := strings.Cut(c.Host, ":"); found {
		return host
	}
	return c.Host
}

func (c *RegistryConfig) Repository(project string) string {
	return fmt.Sprintf("%s/%s/%s", c.Address(), c.Namespace, project)
}

func (c *Config) normalize() {
	base := Default()
	if c.Server.Host == "" {
		c.Server.Host = base.Server.Host
	}
	if c.Server.UFPPort == 0 {
		c.Server.UFPPort = base.Server.UFPPort
	}
	if c.Server.HTTPPort == 0 {
		c.Server.HTTPPort = base.Server.HTTPPort
	}
	if c.Server.DataDir == "" {
		c.Server.DataDir = base.Server.DataDir
	}
	if c.Registry.Port == 0 {
		c.Registry.Port = base.Registry.Port
	}
	if c.Registry.Namespace == "" {
		c.Registry.Namespace = base.Registry.Namespace
	}
	if c.Registry.Image == "" {
		c.Registry.Image = base.Registry.Image
	}
	if c.Registry.Socket == "" {
		c.Registry.Socket = base.Registry.Socket
	}
	if c.Webhook.Path == "" {
		c.Webhook.Path = base.Webhook.Path
	}
}
