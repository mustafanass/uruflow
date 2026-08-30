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

package pipeline

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/link"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/pki"
	"github.com/mustafanass/uruflow/internal/registry"
	"github.com/mustafanass/uruflow/internal/secrets"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/mustafanass/uruflow/internal/ufp"
)

const (
	settleWindow  = 5 * time.Second
	builtDigest   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	builtImage    = "127.0.0.1:5000/uruflow/api@" + builtDigest
	prebuiltImage = "redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	builtCommit   = "0123456789abcdef0123456789abcdef01234567"
)

type fakeAgent struct {
	conn      *ufp.Conn
	failBuild bool
	failRun   bool
	holdBuild chan struct{}
	builds    chan ufp.BuildRequest
	releases  chan ufp.ReleaseRequest
}

func (a *fakeAgent) HandleRequest(request *ufp.Request) (any, error) {
	switch request.Method {
	case ufp.MethodBuildRun:
		var payload ufp.BuildRequest
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		a.builds <- payload
		go a.finishBuild(payload)
		return ufp.Accepted{JobID: payload.JobID}, nil

	case ufp.MethodReleaseRun:
		var payload ufp.ReleaseRequest
		if err := request.Decode(&payload); err != nil {
			return nil, err
		}
		a.releases <- payload
		go a.finishRelease(payload)
		return ufp.Accepted{JobID: payload.JobID}, nil
	}
	return ufp.Accepted{}, nil
}

func (a *fakeAgent) HandleEvent(*ufp.Event) error { return nil }

func (a *fakeAgent) finishBuild(request ufp.BuildRequest) {
	if a.holdBuild != nil {
		<-a.holdBuild
	}
	a.conn.SendEvent(ufp.TopicJobLog, ufp.JobLog{
		JobID: request.JobID, Stage: ufp.StageBuild,
		Stream: ufp.StreamStdout, Line: "building " + request.Project,
	})

	images := make(map[string]string, len(request.Targets))
	commits := make(map[string]string, len(request.Targets))
	primary := ""
	for _, target := range request.Targets {
		images[target.Service] = target.Image + "@" + builtDigest
		commits[target.Service] = builtCommit
		if primary == "" {
			primary = images[target.Service]
		}
	}

	status := ufp.JobStatus{
		JobID: request.JobID, Stage: ufp.StageBuild,
		Status: ufp.StatusSuccess, Image: primary, Images: images,
		Commit: builtCommit, Commits: commits, Digest: builtDigest,
	}
	if a.failBuild {
		status = ufp.JobStatus{JobID: request.JobID, Stage: ufp.StageBuild,
			Status: ufp.StatusFailed, Message: "compile error"}
	}
	a.conn.SendEvent(ufp.TopicJobStatus, status)
}

func (a *fakeAgent) finishRelease(request ufp.ReleaseRequest) {
	status := ufp.JobStatus{JobID: request.JobID, Stage: ufp.StageRelease, Status: ufp.StatusSuccess}
	if a.failRun {
		status = ufp.JobStatus{JobID: request.JobID, Stage: ufp.StageRelease,
			Status: ufp.StatusFailed, Message: "port already bound"}
	}
	a.conn.SendEvent(ufp.TopicJobStatus, status)
}

type harness struct {
	store    *sqlite.Store
	pipeline *Pipeline
	agent    *fakeAgent
	vault    *secrets.Vault
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.UFPPort = 0
	cfg.Server.DataDir = dir
	cfg.Server.Advertise = "127.0.0.1"
	cfg.Registry.Host = "127.0.0.1"

	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}

	authority, err := pki.LoadOrCreateCA(cfg.CACertPath(), cfg.CAKeyPath())
	if err != nil {
		t.Fatalf("ca: %v", err)
	}
	if err := authority.EnsureLeaf(pki.Material{
		CertPath: cfg.ServerCertPath(), KeyPath: cfg.ServerKeyPath(),
		Names: []string{ufp.ServerName, "127.0.0.1"},
	}); err != nil {
		t.Fatalf("server certificate: %v", err)
	}
	caPEM, _ := authority.CertificatePEM()

	store, err := sqlite.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.CreateAgent(&models.Agent{ID: "a1", Name: "node-01", Key: "k",
		Roles: []models.Role{models.RoleBuilder, models.RoleRunner}}); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := store.SaveProject(&models.Project{
		Name: "api", GitURL: "git@host:api.git", Branch: "main",
		Builder: "a1", Runners: []string{"a1"}, AutoDeploy: true,
		Runtime: models.Runtime{Ports: []models.Port{{Host: 8080, Container: 80}}},
	}); err != nil {
		t.Fatalf("save project: %v", err)
	}

	links := link.NewServer(cfg, store)
	links.SetRegistry(ufp.RegistryConfig{
		Host: "127.0.0.1:5000", Username: "uruflow", Password: "secret", CACert: caPEM,
	})
	if err := links.Start(); err != nil {
		t.Fatalf("start link: %v", err)
	}
	t.Cleanup(func() { links.Stop() })

	images := registry.New(registry.Options{
		Address: "127.0.0.1:5000", Namespace: "uruflow", CACert: caPEM,
	}, nil)

	vault, err := secrets.LoadOrCreateVault(filepath.Join(dir, "secrets.key"))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}

	releases := New(store, links, images, vault)
	links.Subscribe(releases)

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(caPEM))

	netConn, err := tls.DialWithDialer(&net.Dialer{Timeout: settleWindow}, "tcp", links.Addr(), &tls.Config{
		RootCAs: pool, ServerName: ufp.ServerName, MinVersion: tls.VersionTLS13,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	hello := ufp.Hello{AgentID: "a1", Hostname: "box", Version: "2.0.0",
		Roles: []ufp.Role{ufp.RoleBuilder, ufp.RoleRunner}}
	conn, _, err := ufp.Dial(netConn, hello, "k")
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	agent := &fakeAgent{
		conn:     conn,
		builds:   make(chan ufp.BuildRequest, 4),
		releases: make(chan ufp.ReleaseRequest, 4),
	}
	go conn.Serve(context.Background(), agent)
	if err := conn.SendEvent(ufp.TopicRegistryReady, ufp.Accepted{}); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "agent to come online", func() bool { return links.Online("a1") })

	return &harness{store: store, pipeline: releases, agent: agent, vault: vault}
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(settleWindow)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (h *harness) await(t *testing.T, releaseID string, status models.Status) *models.Release {
	t.Helper()

	var release *models.Release
	waitFor(t, "release "+releaseID+" to reach "+string(status), func() bool {
		loaded, err := h.store.GetRelease(releaseID)
		if err != nil {
			return false
		}
		release = loaded
		return loaded.Status == status
	})
	return release
}

func TestBuildThenReleaseReachesRunners(t *testing.T) {
	harness := newHarness(t)

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if release.Status != models.StatusBuilding {
		t.Fatalf("release starts as %s, want building", release.Status)
	}

	select {
	case build := <-harness.agent.builds:
		if len(build.Targets) != 1 {
			t.Fatalf("build targets = %+v", build.Targets)
		}
		if build.Targets[0].Image != "127.0.0.1:5000/uruflow/api" {
			t.Fatalf("build target repository = %q", build.Targets[0].Image)
		}
		if len(build.Tags) != 1 || build.Tags[0] != TagLatest {
			t.Fatalf("extra tags = %v", build.Tags)
		}
		if build.Targets[0].Dockerfile != "Dockerfile" || build.Targets[0].Context != "." {
			t.Fatalf("build defaults = %q %q", build.Targets[0].Dockerfile, build.Targets[0].Context)
		}
	case <-time.After(settleWindow):
		t.Fatal("the builder never received build.run")
	}

	select {
	case run := <-harness.agent.releases:
		if len(run.Services) != 1 || run.Services[0].Image != builtImage {
			t.Fatalf("runner received %+v, want the freshly built tag", run.Services)
		}
		if len(run.Services[0].Ports) != 1 || run.Services[0].Ports[0].Host != 8080 {
			t.Fatalf("runtime ports = %+v", run.Services[0].Ports)
		}
	case <-time.After(settleWindow):
		t.Fatal("the runner never received release.run")
	}

	final := harness.await(t, release.ID, models.StatusSucceeded)
	if final.Image != builtImage || final.Commit != builtCommit {
		t.Fatalf("release recorded image %q commit %q", final.Image, final.Commit)
	}
	if len(final.Targets) != 1 || final.Targets[0].Status != models.StatusSucceeded {
		t.Fatalf("targets = %+v", final.Targets)
	}
	if final.Duration <= 0 || final.EndedAt == nil {
		t.Fatalf("release was not closed out: duration=%d ended=%v", final.Duration, final.EndedAt)
	}

	logs, _ := harness.store.ListLogs(release.ID)
	if len(logs) == 0 {
		t.Fatal("build logs were not persisted")
	}
}

func TestBuildOnlyStopsAfterPublishingArtifacts(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Workflow = models.WorkflowBuildOnly
	project.Runners = nil
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-harness.agent.builds:
	case <-time.After(settleWindow):
		t.Fatal("build-only workflow never reached the builder")
	}
	final := harness.await(t, release.ID, models.StatusSucceeded)
	if len(final.Targets) != 0 || final.Image != builtImage {
		t.Fatalf("build-only result = targets:%+v image:%q", final.Targets, final.Image)
	}
	select {
	case run := <-harness.agent.releases:
		t.Fatalf("build-only workflow dispatched release.run: %+v", run)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestReleaseOnlySkipsBuilder(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Workflow = models.WorkflowDeployOnly
	project.GitURL = ""
	project.Branch = ""
	project.Builder = ""
	project.Services = []models.Service{{Name: "cache", Image: prebuiltImage}}
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if release.Status != models.StatusReleasing {
		t.Fatalf("release-only workflow started as %q", release.Status)
	}
	select {
	case build := <-harness.agent.builds:
		t.Fatalf("release-only workflow dispatched build.run: %+v", build)
	case run := <-harness.agent.releases:
		if len(run.Services) != 1 || run.Services[0].Image != prebuiltImage {
			t.Fatalf("release-only services = %+v", run.Services)
		}
	case <-time.After(settleWindow):
		t.Fatal("release-only workflow never reached the runner")
	}
	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestFailedBuildNeverReachesRunners(t *testing.T) {
	harness := newHarness(t)
	harness.agent.failBuild = true

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}

	final := harness.await(t, release.ID, models.StatusFailed)
	if final.Message != "compile error" {
		t.Fatalf("failure message = %q", final.Message)
	}

	select {
	case run := <-harness.agent.releases:
		t.Fatalf("a failed build still rolled out %+v", run.Services)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestFailedRunnerFailsTheRelease(t *testing.T) {
	harness := newHarness(t)
	harness.agent.failRun = true

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}

	final := harness.await(t, release.ID, models.StatusFailed)
	if len(final.Targets) != 1 || final.Targets[0].Status != models.StatusFailed {
		t.Fatalf("targets = %+v", final.Targets)
	}
	if final.Targets[0].Message != "port already bound" {
		t.Fatalf("target message = %q", final.Targets[0].Message)
	}
}

func TestRollbackSkipsTheBuildStage(t *testing.T) {
	harness := newHarness(t)

	first, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	<-harness.agent.builds
	<-harness.agent.releases
	harness.await(t, first.ID, models.StatusSucceeded)

	rollback, err := harness.pipeline.Rollback("api", "")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rollback.Image != builtImage {
		t.Fatalf("rollback image = %q", rollback.Image)
	}

	select {
	case build := <-harness.agent.builds:
		t.Fatalf("rollback rebuilt %q instead of reusing the image", build.Project)
	case run := <-harness.agent.releases:
		if run.Services[0].Image != builtImage {
			t.Fatalf("rollback released %q", run.Services[0].Image)
		}
	case <-time.After(settleWindow):
		t.Fatal("rollback never reached a runner")
	}

	harness.await(t, rollback.ID, models.StatusSucceeded)
}

func TestConcurrentReleasesAreRejected(t *testing.T) {
	harness := newHarness(t)

	first, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	<-harness.agent.builds

	if _, err := harness.pipeline.Trigger("api", "", models.TriggerManual); !errors.Is(err, ErrReleaseInFlight) {
		t.Fatalf("second trigger error = %v, want ErrReleaseInFlight", err)
	}
	if _, err := harness.pipeline.Rollback("api", ""); !errors.Is(err, ErrReleaseInFlight) {
		t.Fatalf("rollback during a release = %v, want ErrReleaseInFlight", err)
	}

	<-harness.agent.releases
	harness.await(t, first.ID, models.StatusSucceeded)

	second, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger once the first settled: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("the second release reused the first release id")
	}
}

func TestConcurrentTriggersOnlyOneWins(t *testing.T) {
	harness := newHarness(t)

	const attempts = 8
	results := make(chan error, attempts)
	for range attempts {
		go func() {
			_, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
			results <- err
		}()
	}

	accepted := 0
	for range attempts {
		if err := <-results; err == nil {
			accepted++
		} else if !errors.Is(err, ErrReleaseInFlight) {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if accepted != 1 {
		t.Fatalf("%d concurrent triggers were accepted, want exactly 1", accepted)
	}
}

func TestTriggerRejectsUnknownProject(t *testing.T) {
	harness := newHarness(t)

	if _, err := harness.pipeline.Trigger("ghost", "", models.TriggerManual); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("error = %v, want ErrProjectNotFound", err)
	}
}

func TestSecretsResolveIntoTheReleaseRequest(t *testing.T) {
	harness := newHarness(t)

	sealed, err := harness.vault.Seal("postgres://user:pass@db/api")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if err := harness.store.SetSecret("api_db", sealed); err != nil {
		t.Fatalf("store secret: %v", err)
	}

	project, _ := harness.store.GetProject("api")
	project.Runtime.Env = map[string]string{
		"DATABASE_URL": "${secret:api_db}",
		"MODE":         "production",
	}
	harness.store.SaveProject(project)

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	<-harness.agent.builds

	select {
	case run := <-harness.agent.releases:
		env := run.Services[0].Env
		if env["DATABASE_URL"] != "postgres://user:pass@db/api" {
			t.Fatalf("secret was not resolved: %q", env["DATABASE_URL"])
		}
		if env["MODE"] != "production" {
			t.Fatalf("plain value was altered: %q", env["MODE"])
		}
	case <-time.After(settleWindow):
		t.Fatal("the runner never received the release")
	}

	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestMissingSecretFailsBeforeBuilding(t *testing.T) {
	harness := newHarness(t)

	project, _ := harness.store.GetProject("api")
	project.Runtime.Env = map[string]string{"DATABASE_URL": "${secret:absent}"}
	harness.store.SaveProject(project)

	_, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if !errors.Is(err, ErrSecretMissing) {
		t.Fatalf("error = %v, want ErrSecretMissing", err)
	}

	select {
	case build := <-harness.agent.builds:
		t.Fatalf("a build was dispatched despite the missing secret: %s", build.JobID)
	case <-time.After(500 * time.Millisecond):
	}

	releases, _ := harness.store.ListReleases(10)
	if len(releases) != 0 {
		t.Fatalf("a release was recorded for a rejected trigger: %+v", releases)
	}
}

func TestMultiServiceBuildsEachAndReleasesTogether(t *testing.T) {
	harness := newHarness(t)

	project, _ := harness.store.GetProject("api")
	project.Services = []models.Service{
		{Name: "app", Dockerfile: "Dockerfile", Context: ".",
			Ports:       []models.Port{{Host: 8080, Container: 80}},
			Healthcheck: &models.Healthcheck{Type: "http", Scheme: "http", Path: "/ready", Port: 80, Interval: 2 * time.Second, Timeout: time.Second, Retries: 5},
			Labels:      map[string]string{"traefik.enable": "true"}},
		{Name: "worker", Dockerfile: "Dockerfile.worker", Command: "./worker"},
		{Name: "cache", Image: prebuiltImage,
			Volumes: []models.Volume{{Source: "/srv/redis", Target: "/data"}}},
	}
	project.Runtime.Env = map[string]string{"SHARED": "yes"}
	harness.store.SaveProject(project)

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}

	select {
	case build := <-harness.agent.builds:
		if len(build.Targets) != 2 {
			t.Fatalf("built %d targets, want 2 (cache is prebuilt): %+v", len(build.Targets), build.Targets)
		}
		byService := map[string]ufp.BuildTarget{}
		for _, target := range build.Targets {
			byService[target.Service] = target
		}
		if byService["app"].Image != "127.0.0.1:5000/uruflow/api-app" {
			t.Errorf("app repository = %q", byService["app"].Image)
		}
		if byService["worker"].Dockerfile != "Dockerfile.worker" {
			t.Errorf("worker dockerfile = %q", byService["worker"].Dockerfile)
		}
		if _, built := byService["cache"]; built {
			t.Error("a prebuilt service was sent to the builder")
		}
	case <-time.After(settleWindow):
		t.Fatal("no build was dispatched")
	}

	select {
	case run := <-harness.agent.releases:
		if len(run.Services) != 3 {
			t.Fatalf("released %d services, want 3: %+v", len(run.Services), run.Services)
		}
		byName := map[string]ufp.ServiceSpec{}
		for _, service := range run.Services {
			byName[service.Name] = service
		}
		if byName["cache"].Image != prebuiltImage {
			t.Errorf("prebuilt image was replaced: %q", byName["cache"].Image)
		}
		if byName["app"].Image != "127.0.0.1:5000/uruflow/api-app@"+builtDigest {
			t.Errorf("app image = %q", byName["app"].Image)
		}
		if byName["worker"].Command != "./worker" {
			t.Errorf("worker command = %q", byName["worker"].Command)
		}
		if byName["app"].Env["SHARED"] != "yes" {
			t.Errorf("project env did not reach the service: %v", byName["app"].Env)
		}
		if byName["app"].Healthcheck == nil || byName["app"].Healthcheck.Path != "/ready" || byName["app"].Healthcheck.Retries != 5 {
			t.Errorf("healthcheck did not reach runner: %+v", byName["app"].Healthcheck)
		}
		if byName["app"].Labels["traefik.enable"] != "true" {
			t.Errorf("labels did not reach runner: %#v", byName["app"].Labels)
		}
		if byName["cache"].Restart != "unless-stopped" {
			t.Errorf("default restart policy missing: %q", byName["cache"].Restart)
		}
	case <-time.After(settleWindow):
		t.Fatal("no release was dispatched")
	}

	final := harness.await(t, release.ID, models.StatusSucceeded)
	if final.Spec.Services[0].Healthcheck == nil || final.Spec.Services[0].Labels["traefik.enable"] != "true" {
		t.Fatalf("release snapshot lost runtime specification: %+v", final.Spec.Services[0])
	}
}

func TestNativeBuildModelReachesBuilderAndRunner(t *testing.T) {
	harness := newHarness(t)
	project, _ := harness.store.GetProject("api")
	project.Networks = map[string]models.NetworkResource{
		"data": {Name: "api-data", Driver: "bridge", Internal: true},
	}
	project.Volumes = map[string]models.VolumeResource{
		"state": {Name: "api-state", Driver: "local"},
	}
	project.Services = []models.Service{
		{Name: "core", Dockerfile: "Dockerfile", DependsOn: []models.Dependency{{Service: "migrate", Condition: models.DependencyCompleted}}, Networks: []models.NetworkAttachment{{Name: "data", Aliases: []string{"core-api"}}}, Ports: []models.Port{{HostIP: "127.0.0.1", Host: 8080, Container: 8080}}, Resources: models.ResourceLimits{MemoryBytes: 256 << 20, CPUs: 1.5, PIDs: 128}, Security: models.Security{NoNewPrivileges: true, ReadOnlyRootFS: true, CapDrop: []string{"ALL"}}, Logging: models.LogConfig{Driver: "json-file", Options: map[string]string{"max-size": "10m"}}, Entrypoint: []string{"/app/core"}, CommandExec: []string{"serve"}},
		{Name: "migrate", GitURL: "git@host:database.git", Branch: "release", Dockerfile: "Dockerfile.migrate", Mode: models.ServiceModeJob, Job: models.Job{Timeout: 2 * time.Minute}, DependsOn: []models.Dependency{{Service: "database", Condition: models.DependencyHealthy}}, Networks: []models.NetworkAttachment{{Name: "data"}}},
		{Name: "database", Image: prebuiltImage, Networks: []models.NetworkAttachment{{Name: "data"}}, Volumes: []models.Volume{{Type: "volume", Source: "state", Target: "/data"}}, Healthcheck: &models.Healthcheck{Type: "command", Command: []string{"CMD", "check-ready"}, Interval: time.Second, Timeout: time.Second, Retries: 3, StartPeriod: 2 * time.Second}},
	}
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	build := <-harness.agent.builds
	if len(build.Targets) != 2 || build.Targets[1].GitURL != "git@host:database.git" || build.Targets[1].Branch != "release" {
		t.Fatalf("build targets = %#v", build.Targets)
	}

	run := <-harness.agent.releases
	if len(run.Services) != 3 || run.Services[0].Name != "database" || run.Services[1].Name != "migrate" || run.Services[2].Name != "core" {
		t.Fatalf("dependency order = %#v", run.Services)
	}
	core := run.Services[2]
	if run.Networks["data"].Name != "api-data" || run.Volumes["state"].Name != "api-state" || core.Ports[0].HostIP != "127.0.0.1" || core.Resources.MemoryBytes != 256<<20 || !core.Security.ReadOnlyRootFS || core.Logging.Options["max-size"] != "10m" || len(core.CommandExec) != 1 {
		t.Fatalf("native core spec = %#v; networks=%#v volumes=%#v", core, run.Networks, run.Volumes)
	}
	if run.Services[1].Mode != models.ServiceModeJob || run.Services[1].JobTimeout != 2*time.Minute || run.Services[0].Healthcheck == nil || run.Services[0].Healthcheck.StartPeriod != 2*time.Second {
		t.Fatalf("job/health model = %#v", run.Services)
	}
	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestBuildStatusFromAnotherAgentIsRejected(t *testing.T) {
	harness := newHarness(t)
	harness.agent.holdBuild = make(chan struct{})

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	build := <-harness.agent.builds
	harness.pipeline.JobStatus("another-agent", ufp.JobStatus{
		JobID: release.ID, Stage: ufp.StageBuild, Status: ufp.StatusSuccess,
		Image: builtImage, Images: map[string]string{"": builtImage}, Digest: builtDigest, Commit: builtCommit,
	})
	loaded, err := harness.store.GetRelease(release.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != models.StatusBuilding || loaded.Image != "" {
		t.Fatalf("forged event changed release: %+v", loaded)
	}
	close(harness.agent.holdBuild)
	_ = build
	<-harness.agent.releases
	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestMutableBuilderArtifactIsRejected(t *testing.T) {
	harness := newHarness(t)
	harness.agent.holdBuild = make(chan struct{})

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	<-harness.agent.builds
	harness.pipeline.JobStatus("a1", ufp.JobStatus{
		JobID: release.ID, Stage: ufp.StageBuild, Status: ufp.StatusSuccess,
		Image:  "127.0.0.1:5000/uruflow/api:latest",
		Images: map[string]string{"": "127.0.0.1:5000/uruflow/api:latest"},
		Commit: builtCommit,
	})
	final := harness.await(t, release.ID, models.StatusFailed)
	if final.Message == "" {
		t.Fatal("invalid artifact failure had no message")
	}
	close(harness.agent.holdBuild)
	select {
	case request := <-harness.agent.releases:
		t.Fatalf("mutable artifact reached runner: %+v", request)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestReleaseUsesTheClaimedProjectSnapshot(t *testing.T) {
	harness := newHarness(t)
	harness.agent.holdBuild = make(chan struct{})

	release, err := harness.pipeline.Trigger("api", "", models.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	<-harness.agent.builds
	project, _ := harness.store.GetProject("api")
	project.Runtime.Ports[0].Host = 9090
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	close(harness.agent.holdBuild)

	select {
	case request := <-harness.agent.releases:
		if request.Services[0].Ports[0].Host != 8080 {
			t.Fatalf("release used edited port %d", request.Services[0].Ports[0].Host)
		}
	case <-time.After(settleWindow):
		t.Fatal("release was not dispatched")
	}
	harness.await(t, release.ID, models.StatusSucceeded)
}

func TestOfflineConfiguredRunnerRejectsTheRelease(t *testing.T) {
	harness := newHarness(t)
	if err := harness.store.CreateAgent(&models.Agent{ID: "a2", Name: "node-02", Key: "k2",
		Roles: []models.Role{models.RoleRunner}}); err != nil {
		t.Fatal(err)
	}
	project, _ := harness.store.GetProject("api")
	project.Runners = append(project.Runners, "a2")
	if err := harness.store.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if _, err := harness.pipeline.Trigger("api", "", models.TriggerManual); !errors.Is(err, ErrRunnerOffline) {
		t.Fatalf("error = %v, want ErrRunnerOffline", err)
	}
	releases, _ := harness.store.ListReleases(10)
	if len(releases) != 0 {
		t.Fatalf("rejected release was persisted: %+v", releases)
	}
}
