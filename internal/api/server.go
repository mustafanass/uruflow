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

package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mustafanass/uruflow/internal/activity"
	"github.com/mustafanass/uruflow/internal/api/handlers"
	"github.com/mustafanass/uruflow/internal/api/middleware"
	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/link"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/pipeline"
	"github.com/mustafanass/uruflow/internal/pki"
	"github.com/mustafanass/uruflow/internal/projects"
	"github.com/mustafanass/uruflow/internal/registry"
	"github.com/mustafanass/uruflow/internal/secrets"
	"github.com/mustafanass/uruflow/internal/services"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const (
	readTimeout     = 10 * time.Second
	writeTimeout    = 10 * time.Second
	idleTimeout     = 60 * time.Second
	registryTimeout = 3 * time.Minute

	interruptedMessage = "interrupted by a server restart"
)

type Server struct {
	catalogCancel context.CancelFunc
	cfg           *config.Config
	store         storage.Store
	link          *link.Server
	registry      *registry.Registry
	pipeline      *pipeline.Pipeline
	vault         *secrets.Vault
	activity      *activity.Feed
	webhook       *services.WebhookService
	caCert        string

	webhookMu     sync.Mutex
	webhookServer *http.Server
	webhookAddr   string
	webhookSecure bool
	webhookRoutes atomic.Pointer[http.Handler]
	webhookTLS    atomic.Pointer[tls.Certificate]

	registryRetain atomic.Int64

	loader   *projects.Loader
	problems []projects.Problem
	mu       sync.RWMutex
}

func NewServer(cfg *config.Config, store storage.Store) (*Server, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}

	if err := validateKeyMaterial(cfg, store); err != nil {
		return nil, err
	}

	caCert, err := preparePKI(cfg)
	if err != nil {
		return nil, err
	}

	engine, err := docker.New(cfg.Registry.Socket)
	if err != nil {
		return nil, fmt.Errorf("uruflow needs docker to host the registry: %w", err)
	}

	vault, err := secrets.LoadOrCreateVault(cfg.SecretKeyPath())
	if err != nil {
		return nil, fmt.Errorf("secret vault: %w", err)
	}
	_, registrySettings, err := loadRegistrySettings(cfg, store, vault)
	if err != nil {
		return nil, fmt.Errorf("registry settings: %w", err)
	}
	password, err := vault.Open(registrySettings.Password)
	if err != nil {
		return nil, fmt.Errorf("open registry password: %w", err)
	}

	images := registry.New(registry.Options{
		Address:      cfg.Registry.Address(),
		Hostname:     cfg.Registry.Hostname(),
		Port:         cfg.Registry.Port,
		Namespace:    cfg.Registry.Namespace,
		Username:     cfg.Registry.Username,
		Password:     password,
		Image:        cfg.Registry.Image,
		DataDir:      cfg.RegistryDataDir(),
		CertPath:     cfg.RegistryCertPath(),
		KeyPath:      cfg.RegistryKeyPath(),
		HtpasswdPath: cfg.HtpasswdPath(),
		CACert:       caCert,
	}, engine)

	feed := activity.New(activity.DefaultCapacity)
	links := link.NewServer(cfg, store)
	links.SetActivity(feed)
	links.SetRegistry(ufp.RegistryConfig{
		Host:     cfg.Registry.Address(),
		Username: cfg.Registry.Username,
		Password: password,
		CACert:   caCert,
	})
	var agentSettings models.AgentSettings
	if _, err := readSetting(store, models.SettingAgents, &agentSettings); err == nil {
		links.SetAgentConfig(ufp.AgentConfig{MetricsSec: agentSettings.MetricsSec, ReconnectSec: agentSettings.ReconnectSec})
	} else if !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("agent settings: %w", err)
	}

	releases := pipeline.New(store, links, images, vault)
	releases.SetActivity(feed)
	links.Subscribe(releases)

	loader := projects.NewLoader(cfg.ConfigDir(), func(name string) (*models.Agent, error) {
		return store.GetAgentByName(name)
	})

	server := &Server{
		cfg:      cfg,
		store:    store,
		link:     links,
		registry: images,
		pipeline: releases,
		vault:    vault,
		activity: feed,
		webhook:  services.NewWebhookService(store, releases),
		caCert:   caCert,
		loader:   loader,
	}
	server.ApplyRegistrySettings(registrySettings)
	return server, nil
}

func validateKeyMaterial(cfg *config.Config, store storage.Store) error {
	_, certErr := os.Stat(cfg.CACertPath())
	_, keyErr := os.Stat(cfg.CAKeyPath())
	if os.IsNotExist(certErr) && os.IsNotExist(keyErr) {
		agents, err := store.ListAgents()
		if err != nil {
			return fmt.Errorf("inspect enrolled agents: %w", err)
		}
		if len(agents) > 0 {
			return fmt.Errorf("certificate authority is missing while agents are enrolled")
		}
	}
	if os.IsNotExist(certErr) != os.IsNotExist(keyErr) {
		return fmt.Errorf("certificate authority material is incomplete")
	}

	if _, err := os.Stat(cfg.SecretKeyPath()); os.IsNotExist(err) {
		stored, listErr := store.ListSecrets()
		if listErr != nil {
			return fmt.Errorf("inspect encrypted secrets: %w", listErr)
		}
		if len(stored) > 0 {
			return fmt.Errorf("secret encryption key is missing while encrypted secrets exist")
		}
		for _, name := range []string{models.SettingWebhook, models.SettingRegistry} {
			if _, settingErr := store.GetSetting(name); settingErr == nil {
				return fmt.Errorf("secret encryption key is missing while encrypted settings exist")
			} else if !errors.Is(settingErr, storage.ErrNotFound) {
				return fmt.Errorf("inspect encrypted settings: %w", settingErr)
			}
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s *Server) Start() error {
	if err := s.reconcile(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), registryTimeout)
	defer cancel()

	logger.Info("[REGISTRY] bringing up %s on %s", s.cfg.Registry.Image, s.cfg.Registry.Address())
	if err := s.registry.Ensure(ctx, func(status string) { logger.Debug("[REGISTRY] %s", status) }); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	logger.Info("[REGISTRY] ready on %s", s.cfg.Registry.Address())

	s.ReloadProjects()

	if err := s.link.Start(); err != nil {
		return err
	}
	catalogCtx, catalogCancel := context.WithCancel(context.Background())
	s.catalogCancel = catalogCancel
	s.registry.StartCatalog(catalogCtx)
	s.pipeline.StartRegistryMaintenance(catalogCtx, s.retainReleases)

	s.warnImportedConfig()
	return s.startWebhooks()
}

func (s *Server) reconcile() error {
	if err := s.store.SetAllAgentsOffline(); err != nil {
		return fmt.Errorf("reset agent state: %w", err)
	}
	if err := s.store.FailUnfinishedReleases(interruptedMessage); err != nil {
		return fmt.Errorf("close out interrupted releases: %w", err)
	}

	logger.Info("[SERVER] previous state reconciled")
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.catalogCancel != nil {
		s.catalogCancel()
	}
	if err := s.pipeline.WaitRegistryMaintenance(ctx); err != nil {
		return err
	}
	s.link.Stop()

	s.webhookMu.Lock()
	defer s.webhookMu.Unlock()
	if s.webhookServer != nil {
		return s.webhookServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) ReloadProjects() int {
	result := s.loader.Load()

	loaded := 0
	for index := range result.Projects {
		if err := s.store.SaveProject(&result.Projects[index]); err != nil {
			result.Problems = append(result.Problems, projects.Problem{
				Path: result.Projects[index].Source, Reason: err})
			continue
		}
		loaded++
	}

	result.Problems = append(result.Problems, s.orphans(result.Projects)...)

	s.mu.Lock()
	s.problems = result.Problems
	s.mu.Unlock()

	if loaded > 0 || len(result.Problems) > 0 {
		logger.Info("[PROJECTS] loaded %d from %s, %d problem(s)",
			loaded, s.loader.Dir(), len(result.Problems))
	}
	for _, problem := range result.Problems {
		logger.Warn("[PROJECTS] %s", problem)
	}

	return loaded
}

func (s *Server) orphans(loaded []models.Project) []projects.Problem {
	known := make(map[string]bool, len(loaded))
	for _, project := range loaded {
		known[project.Name] = true
	}

	stored, err := s.store.ListProjects()
	if err != nil {
		return nil
	}

	orphaned := make([]projects.Problem, 0)
	for _, project := range stored {
		if !project.Managed() || known[project.Name] {
			continue
		}
		orphaned = append(orphaned, projects.Problem{
			Path:   project.Source,
			Reason: fmt.Errorf("%s came from this file, which is gone; restore its environment file to manage this project", project.Name),
		})
	}
	return orphaned
}

func (s *Server) ProjectProblems() []projects.Problem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	problems := make([]projects.Problem, len(s.problems))
	copy(problems, s.problems)
	return problems
}

func (s *Server) Vault() *secrets.Vault { return s.vault }

func (s *Server) Loader() *projects.Loader { return s.loader }

func (s *Server) Activity() *activity.Feed { return s.activity }

func (s *Server) ProjectsDir() string { return s.loader.Dir() }

func (s *Server) Config() *config.Config       { return s.cfg }
func (s *Server) Store() storage.Store         { return s.store }
func (s *Server) Link() *link.Server           { return s.link }
func (s *Server) Registry() *registry.Registry { return s.registry }
func (s *Server) Pipeline() *pipeline.Pipeline { return s.pipeline }

func (s *Server) routes(path string) http.Handler {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.RedirectTrailingSlash = false
	_ = router.SetTrustedProxies(nil)
	router.Use(middleware.Logging(), middleware.Recovery())
	webhook := handlers.NewWebhookHandler(s.webhook)

	router.POST(path, webhook.Handle)
	router.GET("/health", handlers.Health)
	router.HEAD("/health", handlers.Health)

	return router
}

func preparePKI(cfg *config.Config) (string, error) {
	authority, err := pki.LoadOrCreateCA(cfg.CACertPath(), cfg.CAKeyPath())
	if err != nil {
		return "", fmt.Errorf("certificate authority: %w", err)
	}

	serverNames := []string{ufp.ServerName, "localhost", "127.0.0.1"}
	if cfg.Server.Advertise != "" {
		serverNames = append(serverNames, cfg.Server.Advertise)
	}
	if cfg.Web.PublicURL != "" {
		if origin, err := url.Parse(cfg.Web.PublicURL); err == nil {
			serverNames = append(serverNames, origin.Hostname())
		}
	}
	if err := authority.EnsureLeaf(pki.Material{
		CertPath: cfg.ServerCertPath(),
		KeyPath:  cfg.ServerKeyPath(),
		Names:    serverNames,
	}); err != nil {
		return "", fmt.Errorf("server certificate: %w", err)
	}

	registryNames := []string{cfg.Registry.Hostname(), "localhost", "127.0.0.1"}
	if err := authority.EnsureLeaf(pki.Material{
		CertPath: cfg.RegistryCertPath(),
		KeyPath:  cfg.RegistryKeyPath(),
		Names:    registryNames,
	}); err != nil {
		return "", fmt.Errorf("registry certificate: %w", err)
	}

	return authority.CertificatePEM()
}
