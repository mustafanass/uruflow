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
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/pkg/helper"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath = "/etc/uruflow/config.yaml"
	DefaultDataDir    = "/var/lib/uruflow"
	DefaultUFPPort    = 9001
	DefaultHTTPPort   = 9000
	DefaultWebPort    = 9002
	DefaultRegistry   = 5000
	DefaultNamespace  = "uruflow"
	RegistryImage     = "registry:2"
	DatabaseFile      = "uruflow.db"
	LogFile           = "uruflow.log"
)

type Config struct {
	path string

	Server   ServerConfig   `yaml:"server"`
	Registry RegistryConfig `yaml:"registry"`
	Webhook  WebhookConfig  `yaml:"webhook"`
	Web      WebConfig      `yaml:"web,omitempty"`
}

type WebConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	TLS       bool   `yaml:"tls"`
	Cert      string `yaml:"cert,omitempty"`
	Key       string `yaml:"key,omitempty"`
	PublicURL string `yaml:"public_url,omitempty"`
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	UFPPort   int    `yaml:"ufp_port"`
	HTTPPort  int    `yaml:"http_port"`
	DataDir   string `yaml:"data_dir"`
	Advertise string `yaml:"advertise"`
}

type RegistryConfig struct {
	RetainReleases int    `yaml:"retain_releases"`
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	Namespace      string `yaml:"namespace"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	Image          string `yaml:"image"`
	Socket         string `yaml:"socket"`
}

type WebhookConfig struct {
	Host   string `yaml:"host,omitempty"`
	Path   string `yaml:"path"`
	Secret string `yaml:"secret"`
	TLS    bool   `yaml:"tls"`
	Cert   string `yaml:"cert,omitempty"`
	Key    string `yaml:"key,omitempty"`
}

func Default() *Config {
	cfg := defaults()
	cfg.Registry.Password = helper.GenerateToken()
	cfg.Webhook.Secret = helper.GenerateSecret()
	return cfg
}

func defaults() *Config {
	return &Config{
		Web: WebConfig{Host: "127.0.0.1", Port: DefaultWebPort},
		Server: ServerConfig{
			Host:     "0.0.0.0",
			UFPPort:  DefaultUFPPort,
			HTTPPort: DefaultHTTPPort,
			DataDir:  DefaultDataDir,
		},
		Registry: RegistryConfig{
			RetainReleases: 10,
			Port:           DefaultRegistry,
			Namespace:      DefaultNamespace,
			Username:       DefaultNamespace,
			Image:          RegistryImage,
			Socket:         "/var/run/docker.sock",
		},
		Webhook: WebhookConfig{
			Path: "/webhook",
			TLS:  true,
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := defaults()
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
	if c.Registry.RetainReleases < 0 {
		return fmt.Errorf("registry.retain_releases must be zero or greater")
	}
	if c.Web.PublicURL != "" {
		u, err := url.Parse(c.Web.PublicURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return fmt.Errorf("web.public_url must be an HTTP or HTTPS origin")
		}
		if u.Scheme == "http" {
			host := u.Hostname()
			ip := net.ParseIP(host)
			if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
				return fmt.Errorf("web.public_url must use HTTPS outside localhost")
			}
		}
	}
	if !strings.HasPrefix(c.Webhook.Path, "/") || strings.ContainsAny(c.Webhook.Path, ":*{}?#\\\r\n") {
		return fmt.Errorf("webhook.path must be a literal absolute URL path")
	}
	if c.Webhook.Path == "/api" || strings.HasPrefix(c.Webhook.Path, "/api/") {
		return fmt.Errorf("webhook.path cannot use the operator API namespace")
	}
	if c.Registry.Host == "" {
		return fmt.Errorf("registry.host is required: agents must be able to reach the registry by this name")
	}
	if strings.ContainsAny(c.Registry.Host, `/\\`) || strings.Contains(c.Registry.Host, "..") {
		return fmt.Errorf("registry.host is invalid")
	}
	if c.Registry.Host != strings.ToLower(c.Registry.Host) {
		return fmt.Errorf("registry.host must be lowercase")
	}
	if !models.ValidResourceName(c.Registry.Namespace) {
		return fmt.Errorf("registry.namespace is invalid")
	}
	if c.Registry.Username == "" || strings.ContainsAny(c.Registry.Username, ":\r\n") {
		return fmt.Errorf("registry.username is invalid")
	}
	if len(c.Registry.Password) < 16 {
		return fmt.Errorf("registry.password must contain at least 16 characters")
	}
	for label, port := range map[string]int{
		"server.ufp_port":  c.Server.UFPPort,
		"server.http_port": c.Server.HTTPPort,
		"registry.port":    c.Registry.Port,
		"web.port":         c.Web.Port,
	} {
		if port < 1 || port > 65535 {
			return fmt.Errorf("%s is outside the valid range", label)
		}
	}
	if c.Server.Advertise == "" {
		return fmt.Errorf("server.advertise is required: agents dial the server by this name")
	}
	if len(c.Webhook.Secret) < 32 {
		return fmt.Errorf("webhook.secret must contain at least 32 characters")
	}
	if !strings.HasPrefix(c.Webhook.Path, "/") || strings.ContainsAny(c.Webhook.Path, " \t\r\n") {
		return fmt.Errorf("webhook.path is invalid")
	}
	if c.Web.Port == c.Server.HTTPPort || c.Web.Port == c.Server.UFPPort || c.Web.Port == c.Registry.Port {
		return fmt.Errorf("web.port must be separate from webhook, UFP and registry ports")
	}
	if (c.Web.Cert == "") != (c.Web.Key == "") {
		return fmt.Errorf("web.cert and web.key must be set together")
	}
	if !c.Web.TLS && c.Web.Cert != "" {
		return fmt.Errorf("web certificates require web.tls")
	}
	if !c.Web.TLS {
		host := strings.Trim(c.Web.Host, "[]")
		address := net.ParseIP(host)
		if host != "localhost" && (address == nil || !address.IsLoopback()) {
			return fmt.Errorf("web.tls may only be disabled on a loopback web.host")
		}
	}
	if (c.Webhook.Cert == "") != (c.Webhook.Key == "") {
		return fmt.Errorf("webhook.cert and webhook.key must be set together")
	}
	if !c.Webhook.TLS && c.Webhook.Cert != "" {
		return fmt.Errorf("webhook.cert and webhook.key require webhook.tls")
	}
	if !c.Webhook.TLS {
		host := strings.Trim(c.webhookHost(), "[]")
		address := net.ParseIP(host)
		if host != "localhost" && (address == nil || !address.IsLoopback()) {
			return fmt.Errorf("webhook.tls may only be disabled on a loopback server.host")
		}
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
	return fmt.Sprintf("%s:%d", c.webhookHost(), c.Server.HTTPPort)
}

func (c *Config) WebhookCertificatePaths() (string, string) {
	if c.Webhook.Cert != "" {
		return c.Webhook.Cert, c.Webhook.Key
	}
	return c.ServerCertPath(), c.ServerKeyPath()
}

func (c *Config) webhookHost() string {
	if c.Webhook.Host != "" {
		return c.Webhook.Host
	}
	return c.Server.Host
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
	base := defaults()
	if c.Web.Host == "" {
		c.Web.Host = base.Web.Host
	}
	if c.Web.Port == 0 {
		c.Web.Port = base.Web.Port
	}
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

func (c *Config) WebAddr() string { return net.JoinHostPort(c.Web.Host, fmt.Sprint(c.Web.Port)) }
func (c *Config) WebCertificatePaths() (string, string) {
	if c.Web.Cert != "" {
		return c.Web.Cert, c.Web.Key
	}
	return c.ServerCertPath(), c.ServerKeyPath()
}
