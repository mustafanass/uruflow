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

package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/mustafanass/uruflow/internal/agent/builder"
	"github.com/mustafanass/uruflow/internal/agent/config"
	"github.com/mustafanass/uruflow/internal/agent/metrics"
	"github.com/mustafanass/uruflow/internal/agent/runner"
	"github.com/mustafanass/uruflow/internal/docker"
	"github.com/mustafanass/uruflow/internal/ufp"
	"github.com/mustafanass/uruflow/pkg/logger"
)

const dialTimeout = 10 * time.Second

var _ ufp.Handler = (*Daemon)(nil)

type Daemon struct {
	cfg     *config.Config
	docker  *docker.Client
	metrics *metrics.Collector
	builder *builder.Builder
	runner  *runner.Runner

	conn   *ufp.Conn
	connMu sync.RWMutex

	registry   ufp.RegistryConfig
	registryMu sync.RWMutex

	streams  map[string]context.CancelFunc
	streamMu sync.Mutex

	stop chan struct{}
	once sync.Once
}

func New(cfg *config.Config) (*Daemon, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	if err := logger.Init(cfg.LogFile, "info"); err != nil {
		return nil, fmt.Errorf("initialise logger: %w", err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	logger.Info("[AGENT] uruflow-agent v%s roles=%v", config.Version, cfg.Roles)

	engine, err := docker.New(cfg.Docker.Socket)
	if err != nil {
		return nil, err
	}

	daemon := &Daemon{
		cfg:     cfg,
		docker:  engine,
		metrics: metrics.NewCollector(),
		streams: make(map[string]context.CancelFunc),
		stop:    make(chan struct{}),
	}
	daemon.runner = runner.New(engine, daemon.registryAuth)

	if cfg.HasRole(ufp.RoleBuilder) {
		daemon.builder = builder.New(cfg.WorkDir())
	}

	return daemon, nil
}

func (d *Daemon) Run() error {
	if err := d.writePid(); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}
	defer d.removePid()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		logger.Info("[AGENT] shutdown requested")
		d.shutdown()
	}()

	reconnect := time.Duration(d.cfg.Server.ReconnectSec) * time.Second

	for {
		select {
		case <-d.stop:
			logger.Info("[AGENT] stopped")
			return nil
		default:
		}

		if err := d.session(); err != nil {
			logger.Warn("[AGENT] %v", err)
		}

		select {
		case <-d.stop:
			logger.Info("[AGENT] stopped")
			return nil
		case <-time.After(reconnect):
		}
	}
}

func (d *Daemon) session() error {
	netConn, err := d.dial()
	if err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	hello := ufp.Hello{
		AgentID:  d.cfg.AgentID,
		Hostname: hostname,
		Version:  config.Version,
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
		Roles:    d.cfg.Roles,
	}

	conn, welcome, err := ufp.Dial(netConn, hello, d.cfg.Key)
	if err != nil {
		netConn.Close()
		return fmt.Errorf("handshake: %w", err)
	}

	d.connMu.Lock()
	d.conn = conn
	d.connMu.Unlock()

	logger.Info("[AGENT] connected as %s (%s)", welcome.Name, welcome.ServerVersion)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.watchShutdown(ctx, conn)
	go d.reportMetrics(ctx)

	err = conn.Serve(ctx, d)

	d.connMu.Lock()
	d.conn = nil
	d.connMu.Unlock()
	d.cancelStreams()

	logger.Warn("[AGENT] link closed, reconnecting in %ds", d.cfg.Server.ReconnectSec)
	return err
}

func (d *Daemon) dial() (net.Conn, error) {
	address := fmt.Sprintf("%s:%d", d.cfg.Server.Host, d.cfg.Server.Port)

	authority, err := os.ReadFile(d.cfg.Server.CACert)
	if err != nil {
		return nil, fmt.Errorf("read CA %s: %w", d.cfg.Server.CACert, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(authority) {
		return nil, fmt.Errorf("CA %s holds no usable certificate", d.cfg.Server.CACert)
	}

	dialer := &net.Dialer{Timeout: dialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		RootCAs:    pool,
		ServerName: ufp.ServerName,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address, err)
	}

	return conn, nil
}

func (d *Daemon) watchShutdown(ctx context.Context, conn *ufp.Conn) {
	select {
	case <-ctx.Done():
	case <-d.stop:
		conn.SendGoodbye("agent shutting down")
		conn.Close()
	}
}

func (d *Daemon) shutdown() {
	d.once.Do(func() { close(d.stop) })
}

func (d *Daemon) send(topic string, payload any) {
	d.connMu.RLock()
	conn := d.conn
	d.connMu.RUnlock()

	if conn == nil {
		return
	}
	if err := conn.SendEvent(topic, payload); err != nil {
		logger.Debug("[AGENT] publish %s failed: %v", topic, err)
	}
}
