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
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
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
	cfg      *config.Config
	store    storage.Store
	link     *link.Server
	registry *registry.Registry
	pipeline *pipeline.Pipeline
	vault    *secrets.Vault
	webhook  *services.WebhookService
	http     *http.Server
	caCert   string

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

	images := registry.New(registry.Options{
		Address:      cfg.Registry.Address(),
		Hostname:     cfg.Registry.Hostname(),
		Port:         cfg.Registry.Port,
		Namespace:    cfg.Registry.Namespace,
		Username:     cfg.Registry.Username,
		Password:     cfg.Registry.Password,
		Image:        cfg.Registry.Image,
		DataDir:      cfg.RegistryDataDir(),
		CertPath:     cfg.RegistryCertPath(),
		KeyPath:      cfg.RegistryKeyPath(),
		HtpasswdPath: cfg.HtpasswdPath(),
		CACert:       caCert,
	}, engine)

	links := link.NewServer(cfg, store)
	links.SetRegistry(ufp.RegistryConfig{
		Host:     cfg.Registry.Address(),
		Username: cfg.Registry.Username,
		Password: cfg.Registry.Password,
		CACert:   caCert,
	})

	vault, err := secrets.LoadOrCreateVault(cfg.SecretKeyPath())
	if err != nil {
		return nil, fmt.Errorf("secret vault: %w", err)
	}

	releases := pipeline.New(store, links, images, vault)
	links.Subscribe(releases)

	loader := projects.NewLoader(cfg.ConfigDir(), func(name string) (*models.Agent, error) {
		return store.GetAgentByName(name)
	})

	return &Server{
		cfg:      cfg,
		store:    store,
		link:     links,
		registry: images,
		pipeline: releases,
		vault:    vault,
		webhook:  services.NewWebhookService(cfg, store, releases),
		caCert:   caCert,
		loader:   loader,
	}, nil
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

	s.http = &http.Server{
		Addr:         s.cfg.HTTPAddr(),
		Handler:      s.routes(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
		TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS13},
	}

	scheme := "https"
	if !s.cfg.Webhook.TLS {
		scheme = "http"
	}
	logger.Info("[HTTP] webhooks on %s://%s%s", scheme, s.cfg.HTTPAddr(), s.cfg.Webhook.Path)

	go func() {
		var err error
		if s.cfg.Webhook.TLS {
			certPath, keyPath := s.cfg.WebhookCertificatePaths()
			err = s.http.ListenAndServeTLS(certPath, keyPath)
		} else {
			err = s.http.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Error("[HTTP] %v", err)
		}
	}()

	return nil
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
	s.link.Stop()

	if s.http != nil {
		return s.http.Shutdown(ctx)
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
			Reason: fmt.Errorf("%s came from this file, which is gone — press d to remove the project", project.Name),
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

func (s *Server) ProjectsDir() string { return s.loader.Dir() }

func (s *Server) Config() *config.Config       { return s.cfg }
func (s *Server) Store() storage.Store         { return s.store }
func (s *Server) Link() *link.Server           { return s.link }
func (s *Server) Registry() *registry.Registry { return s.registry }
func (s *Server) Pipeline() *pipeline.Pipeline { return s.pipeline }
func (s *Server) CACertificate() string        { return s.caCert }

func (s *Server) routes() http.Handler {
	router := mux.NewRouter()
	webhook := handlers.NewWebhookHandler(s.webhook)

	router.HandleFunc(s.cfg.Webhook.Path, webhook.Handle).Methods(http.MethodPost)
	router.HandleFunc("/health", handlers.Health).Methods(http.MethodGet)

	return middleware.Recovery(middleware.Logging(router))
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
