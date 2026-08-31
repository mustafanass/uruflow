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
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/internal/version"
	"gopkg.in/yaml.v3"
)

const (
	Version              = version.Current
	DefaultReconnectSecs = 5
	DefaultMetricsSecs   = 10
)

var (
	DefaultConfigPath string
	DefaultDataDir    string
	DefaultPidFile    string
	DefaultLogFile    string
	DefaultCACert     string
)

func init() {
	if runtime.GOOS == "windows" || os.Geteuid() != 0 {
		home, _ := os.UserHomeDir()
		DefaultConfigPath = filepath.Join(home, ".config", "uruflow", "agent.yaml")
		DefaultDataDir = filepath.Join(home, ".local", "share", "uruflow-agent")
		DefaultPidFile = filepath.Join(DefaultDataDir, "agent.pid")
		DefaultLogFile = filepath.Join(DefaultDataDir, "agent.log")
		DefaultCACert = filepath.Join(home, ".config", "uruflow", "ca.crt")
		return
	}

	DefaultConfigPath = "/etc/uruflow/agent.yaml"
	DefaultDataDir = "/var/lib/uruflow-agent"
	DefaultPidFile = "/var/run/uruflow-agent.pid"
	DefaultLogFile = "/var/log/uruflow-agent.log"
	DefaultCACert = "/etc/uruflow/ca.crt"
}

type Config struct {
	AgentID string       `yaml:"agent_id"`
	Key     string       `yaml:"key"`
	Roles   []ufp.Role   `yaml:"roles"`
	DataDir string       `yaml:"data_dir"`
	PidFile string       `yaml:"pid_file"`
	LogFile string       `yaml:"log_file"`
	Server  ServerConfig `yaml:"server"`
	Docker  DockerConfig `yaml:"docker"`
}

type ServerConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	CACert       string `yaml:"ca_cert"`
	ReconnectSec int    `yaml:"reconnect_sec"`
	MetricsSec   int    `yaml:"metrics_sec"`
}

type DockerConfig struct {
	Socket string `yaml:"socket"`
}

func Default() *Config {
	return &Config{
		Roles:   []ufp.Role{ufp.RoleRunner},
		DataDir: DefaultDataDir,
		PidFile: DefaultPidFile,
		LogFile: DefaultLogFile,
		Server: ServerConfig{
			Port:         9001,
			CACert:       DefaultCACert,
			ReconnectSec: DefaultReconnectSecs,
			MetricsSec:   DefaultMetricsSecs,
		},
		Docker: DockerConfig{Socket: docker.DefaultSocket},
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &Error{Path: path, Err: err}
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, &Error{Path: path, Err: err}
	}

	return cfg, nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultConfigPath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return &Error{Path: path, Err: err}
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func (c *Config) Validate() error {
	if c.AgentID == "" {
		return errors.New("agent_id is required")
	}
	if len(c.Key) < 32 {
		return errors.New("key must contain at least 32 characters")
	}
	if c.Server.Host == "" {
		return errors.New("server.host is required")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return errors.New("server.port is outside the valid range")
	}
	if c.Server.ReconnectSec < 1 {
		return errors.New("server.reconnect_sec must be positive")
	}
	if c.Server.MetricsSec < 1 {
		return errors.New("server.metrics_sec must be positive")
	}
	if len(c.Roles) == 0 {
		return errors.New("at least one role is required (builder, runner)")
	}
	seen := make(map[ufp.Role]bool, len(c.Roles))
	for _, role := range c.Roles {
		if !role.Valid() {
			return errors.New("unknown role: " + string(role))
		}
		if seen[role] {
			return errors.New("duplicate role: " + string(role))
		}
		seen[role] = true
	}
	return nil
}

func (c *Config) HasRole(role ufp.Role) bool {
	return ufp.HasRole(c.Roles, role)
}

func (c *Config) WorkDir() string {
	return filepath.Join(c.DataDir, "sources")
}

type Error struct {
	Path string
	Err  error
}

func (e *Error) Error() string {
	switch {
	case os.IsNotExist(e.Err):
		return "agent config not found at " + e.Path
	case os.IsPermission(e.Err):
		return "permission denied reading " + e.Path
	default:
		return "agent config error: " + e.Err.Error()
	}
}

func (e *Error) Unwrap() error { return e.Err }
