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

package views

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mustafanass/uruflow/internal/api"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/projects"
	"github.com/mustafanass/uruflow/internal/tui/components"
	"github.com/mustafanass/uruflow/internal/tui/theme"
	"gopkg.in/yaml.v3"
)

type projectMode int

const (
	projectList projectMode = iota
	projectEdit
	projectConfirmDelete
	projectConfirmDeploy
	projectConfirmRollback
	projectConfirmStop
)

const (
	fieldKind = iota
	fieldName
	fieldEnvName
	fieldGitURL
	fieldBranch
	fieldDockerfile
	fieldContext
	fieldBuilder
	fieldRunners
	fieldPorts
	fieldVolumes
	fieldNetwork
	fieldAuto
)

const (
	tabSettings = iota
	tabVariables
	tabServices
	tabYAML
)

const (
	serviceTabSettings = iota
	serviceTabRuntime
	serviceTabHealth
	serviceTabTiming
	serviceTabBuildArgs
	serviceTabEnv
	serviceTabLabels
	serviceTabCount
)

const (
	serviceFieldName = iota
	serviceFieldSource
	serviceFieldImage
	serviceFieldDockerfile
	serviceFieldContext
)

const (
	runtimeFieldCommand = iota
	runtimeFieldRestart
	runtimeFieldPorts
	runtimeFieldVolumes
	runtimeFieldNetwork
)

const (
	healthFieldType = iota
	healthFieldScheme
	healthFieldPath
	healthFieldPort
)

const (
	timingFieldInterval = iota
	timingFieldTimeout
	timingFieldRetries
	timingFieldStableFor
)

const (
	detailOverview = iota
	detailVariables
	detailConfig
	detailTabCount
)

const (
	kindStandalone = "standalone"
	kindFile       = "file"
	defaultEnvName = "prod"
)

type Projects struct {
	Base
	server   *api.Server
	mode     projectMode
	cursor   int
	projects []models.Project
	last     map[string]models.Release

	form      *components.Form
	variables *components.Sheet
	yaml      *components.Sheet
	tab       int
	detailTab int
	editing   string
	discard   bool

	services       []models.Service
	serviceCursor  int
	serviceOpen    bool
	serviceEditing int
	serviceTab     int
	serviceDirty   bool
	serviceForm    *components.Form
	runtimeForm    *components.Form
	healthForm     *components.Form
	timingForm     *components.Form
	serviceBuild   *components.Sheet
	serviceEnv     *components.Sheet
	serviceLabels  *components.Sheet
}

func NewProjects(server *api.Server) *Projects {
	return &Projects{
		server: server,
		last:   make(map[string]models.Release),
		form: components.NewForm(
			components.NewChoice("stored as", "standalone lives in uruflow, file writes projects/<name>/", false),
			components.NewField("name", "api", "also the image repository name", false),
			components.NewField("environment", defaultEnvName, "file mode only: dev, stg, prod …", true),
			components.NewField("git url", "git@github.com:acme/api.git", "the builder clones this", false),
			components.NewField("branch", "main", "branch that webhooks deploy", false),
			components.NewField("dockerfile", "Dockerfile", "path inside the repository", true),
			components.NewField("context", ".", "build context inside the repository", true),
			components.NewChoice("builder", "agent that builds and pushes the image", false),
			components.NewMulti("runners", "agents that pull and run it", false),
			components.NewField("ports", "8080:80", "host:container, comma separated", true),
			components.NewField("volumes", "/srv/data:/data", "source:target[:ro], comma separated", true),
			components.NewField("network", "bridge", "docker network for the container", true),
			components.NewToggle("auto deploy", "deploy automatically on webhook push"),
		),
		serviceForm: components.NewForm(
			components.NewField("name", "api", "unique service name", false),
			components.NewChoice("source", "build from source or use an immutable digest", false),
			components.NewField("image", "repository@sha256:…", "required for image source", true),
			components.NewField("dockerfile", "Dockerfile", "path inside the repository", true),
			components.NewField("context", ".", "build context inside the repository", true),
		),
		runtimeForm: components.NewForm(
			components.NewField("command", "./server", "container command override", true),
			components.NewField("restart", "unless-stopped", "docker restart policy", true),
			components.NewField("ports", "8080:80", "host:container, comma separated", true),
			components.NewField("volumes", "/srv/data:/data", "source:target[:ro], comma separated", true),
			components.NewField("network", "bridge", "docker network", true),
		),
		healthForm: components.NewForm(
			components.NewChoice("type", "native readiness policy", false),
			components.NewChoice("scheme", "http or https", true),
			components.NewField("path", "/health", "HTTP request path", true),
			components.NewField("port", "8080", "container port", true),
		),
		timingForm: components.NewForm(
			components.NewField("interval", "5s", "HTTP/TCP delay between attempts", true),
			components.NewField("timeout", "3s", "HTTP/TCP per-attempt timeout", true),
			components.NewField("retries", "10", "HTTP/TCP finite attempt count", true),
			components.NewField("stable for", "5s", "running type only", true),
		),
	}
}

func (p *Projects) Init() tea.Cmd {
	p.mode = projectList
	p.reload()
	return nil
}

func (p *Projects) Capturing() bool { return p.mode != projectList }

func (p *Projects) reload() {
	projectsList, err := p.server.Store().ListProjects()
	if err != nil {
		p.Fail(fmt.Errorf("load projects: %w", err))
		return
	}
	p.projects = projectsList
	if p.cursor >= len(p.projects) {
		p.cursor = 0
	}

	for _, project := range p.projects {
		releases, err := p.server.Store().ListReleasesByProject(project.Name, 1)
		if err != nil {
			p.Fail(fmt.Errorf("load releases for %s: %w", project.Name, err))
			continue
		}
		if len(releases) > 0 {
			p.last[project.Name] = releases[0]
		}
	}
}

func (p *Projects) selected() *models.Project {
	if p.cursor < 0 || p.cursor >= len(p.projects) {
		return nil
	}
	return &p.projects[p.cursor]
}

func (p *Projects) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok {
		return p.key(key)
	}
	p.reload()
	return nil
}

func (p *Projects) key(msg tea.KeyMsg) tea.Cmd {
	switch p.mode {
	case projectEdit:
		return p.editKey(msg)
	case projectConfirmDelete:
		return p.confirm(msg, p.remove)
	case projectConfirmDeploy:
		return p.confirm(msg, p.deploy)
	case projectConfirmRollback:
		return p.confirm(msg, p.rollback)
	case projectConfirmStop:
		return p.confirm(msg, p.stop)
	}

	switch msg.String() {
	case "up", "k":
		p.cursor = move(p.cursor, -1, len(p.projects))
	case "down", "j":
		p.cursor = move(p.cursor, 1, len(p.projects))
	case "n":
		p.openCreate()
	case "e":
		p.openEdit()
	case "d":
		if p.selected() != nil {
			p.mode = projectConfirmDelete
		}
	case "enter":
		if p.selected() != nil {
			p.mode = projectConfirmDeploy
		}
	case "r":
		if p.selected() != nil {
			p.mode = projectConfirmRollback
		}
	case "s":
		if p.selected() != nil {
			p.mode = projectConfirmStop
		}
	case "ctrl+t":
		p.detailTab = (p.detailTab + 1) % detailTabCount
	case "R":
		p.Clear()
		loaded := p.server.ReloadProjects()
		p.reload()
		p.Notify(fmt.Sprintf("reloaded %d project file(s) from %s", loaded, p.server.ProjectsDir()), "success")
	}
	return nil
}

func (p *Projects) openCreate() {
	p.Clear()
	p.form.Reset()
	p.refreshAgentOptions()
	p.form.Field(fieldKind).SetOptions([]components.Option{
		{Value: kindStandalone, Label: kindStandalone},
		{Value: kindFile, Label: kindFile},
	})
	p.form.Field(fieldEnvName).Set(defaultEnvName)

	p.variables = components.NewSheet("variables", "")
	p.variables.Load("")
	p.yaml = components.NewSheet("config", "")
	p.yaml.Load("")
	p.services = nil
	p.serviceDirty = false
	p.resetServiceEditor()

	p.editing = ""
	p.tab = tabSettings
	p.discard = false
	p.sizeSheets()
	p.mode = projectEdit
}

func (p *Projects) openEdit() {
	project := p.selected()
	if project == nil {
		return
	}

	p.Clear()
	p.form.Reset()
	p.refreshAgentOptions()
	p.form.Field(fieldKind).SetOptions([]components.Option{
		{Value: kindStandalone, Label: kindStandalone},
		{Value: kindFile, Label: kindFile},
	})
	p.load(project)

	p.variables = components.NewSheet("variables", "")
	p.yaml = components.NewSheet("config", "")
	p.services = append([]models.Service(nil), project.Services...)
	p.serviceDirty = false
	p.resetServiceEditor()

	if project.Managed() {
		p.variables.Path = projects.EnvPathFor(project.Source)
		p.yaml.Path = project.Source
		variables, err := readFile(p.variables.Path)
		if err != nil && !os.IsNotExist(err) {
			p.Fail(fmt.Errorf("read variables: %w", err))
		}
		config, err := readFile(p.yaml.Path)
		if err != nil {
			p.Fail(fmt.Errorf("read config: %w", err))
		}
		p.variables.Load(variables)
		p.yaml.Load(config)
	} else {
		p.variables.Load(projects.FormatDotEnv(project.Runtime.Env))
		p.yaml.Load("")
	}

	p.editing = project.Name
	p.tab = tabSettings
	p.discard = false
	p.sizeSheets()
	p.mode = projectEdit
}

func (p *Projects) sizeSheets() {
	width := components.CardWidth(p.Width)
	height := p.TableHeight(12)
	p.variables.Resize(width, height)
	p.yaml.Resize(width, height)
	if p.serviceBuild != nil {
		p.serviceBuild.Resize(width, height)
		p.serviceEnv.Resize(width, height)
		p.serviceLabels.Resize(width, height)
	}
}

func (p *Projects) fileMode() bool {
	return p.form.Value(fieldKind) == kindFile
}

func (p *Projects) tabs() []components.TabItem {
	items := []components.TabItem{
		{Label: "settings"},
		{Label: "variables", Dirty: p.variables.Dirty()},
		{Label: "services", Dirty: p.serviceDirty},
	}
	if p.fileMode() {
		items = append(items, components.TabItem{Label: "config", Dirty: p.yaml.Dirty()})
	}
	return items
}

func (p *Projects) editKey(msg tea.KeyMsg) tea.Cmd {
	if p.serviceOpen {
		return p.serviceKey(msg)
	}
	if msg.String() != "esc" {
		p.discard = false
	}

	switch msg.String() {
	case "esc":
		if p.dirty() && !p.discard {
			p.discard = true
			p.Notify("unsaved changes — ctrl+s to save, esc again to discard them", "warn")
			return nil
		}
		p.discard = false
		p.mode = projectList
		p.reload()
		return nil

	case "ctrl+s":
		p.save()
		return nil

	case "ctrl+t":
		p.tab = (p.tab + 1) % len(p.tabs())
		return nil
	}

	switch p.tab {
	case tabVariables:
		return p.variables.Update(msg)
	case tabServices:
		return p.servicesKey(msg)
	case tabYAML:
		return p.yaml.Update(msg)
	}

	switch msg.String() {
	case "tab", "down":
		p.form.Next()
		return nil
	case "shift+tab", "up":
		p.form.Previous()
		return nil
	}
	return p.form.Update(msg)
}

func (p *Projects) dirty() bool {
	return p.variables.Dirty() || p.yaml.Dirty() || p.serviceDirty
}

func (p *Projects) resetServiceEditor() {
	p.serviceCursor = 0
	p.serviceOpen = false
	p.serviceEditing = -1
	p.serviceTab = serviceTabSettings
	p.serviceForm.Field(serviceFieldSource).SetOptions([]components.Option{
		{Value: "built", Label: "built"},
		{Value: "image", Label: "image"},
	})
	p.healthForm.Field(healthFieldType).SetOptions([]components.Option{
		{Value: "none", Label: "none"},
		{Value: "http", Label: "http"},
		{Value: "tcp", Label: "tcp"},
		{Value: "running", Label: "running"},
	})
	p.healthForm.Field(healthFieldScheme).SetOptions([]components.Option{
		{Value: "http", Label: "http"},
		{Value: "https", Label: "https"},
	})
	p.serviceBuild = components.NewSheet("build args", "")
	p.serviceEnv = components.NewSheet("environment", "")
	p.serviceLabels = components.NewSheet("labels", "")
	p.serviceBuild.Load("")
	p.serviceEnv.Load("")
	p.serviceLabels.Load("")
}

func (p *Projects) servicesKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		p.serviceCursor = move(p.serviceCursor, -1, len(p.services))
	case "down", "j":
		p.serviceCursor = move(p.serviceCursor, 1, len(p.services))
	case "n":
		p.openService(-1)
	case "e", "enter":
		if p.serviceCursor < len(p.services) {
			p.openService(p.serviceCursor)
		}
	case "d":
		if p.serviceCursor < len(p.services) {
			p.services = append(p.services[:p.serviceCursor], p.services[p.serviceCursor+1:]...)
			p.serviceCursor = move(p.serviceCursor, 0, len(p.services))
			p.serviceDirty = true
		}
	}
	return nil
}

func (p *Projects) openService(index int) {
	p.serviceForm.Reset()
	p.runtimeForm.Reset()
	p.healthForm.Reset()
	p.timingForm.Reset()
	p.serviceForm.Field(serviceFieldSource).SetOptions([]components.Option{{Value: "built", Label: "built"}, {Value: "image", Label: "image"}})
	p.healthForm.Field(healthFieldType).SetOptions([]components.Option{{Value: "none", Label: "none"}, {Value: "http", Label: "http"}, {Value: "tcp", Label: "tcp"}, {Value: "running", Label: "running"}})
	p.healthForm.Field(healthFieldScheme).SetOptions([]components.Option{{Value: "http", Label: "http"}, {Value: "https", Label: "https"}})
	p.serviceBuild.Load("")
	p.serviceEnv.Load("")
	p.serviceLabels.Load("")
	p.serviceEditing = index
	p.serviceTab = serviceTabSettings
	p.serviceOpen = true
	if index >= 0 && index < len(p.services) {
		p.loadService(p.services[index])
	}
	p.sizeSheets()
}

func (p *Projects) loadService(service models.Service) {
	source := "built"
	if !service.Built() {
		source = "image"
	}
	values := map[int]string{
		serviceFieldName: service.Name, serviceFieldSource: source, serviceFieldImage: service.Image,
		serviceFieldDockerfile: service.Dockerfile, serviceFieldContext: service.Context,
	}
	for index, value := range values {
		p.serviceForm.Field(index).Set(value)
	}
	p.runtimeForm.Field(runtimeFieldCommand).Set(service.Command)
	p.runtimeForm.Field(runtimeFieldRestart).Set(service.Restart)
	p.runtimeForm.Field(runtimeFieldPorts).Set(formatPorts(service.Ports))
	p.runtimeForm.Field(runtimeFieldVolumes).Set(formatVolumes(service.Volumes))
	p.runtimeForm.Field(runtimeFieldNetwork).Set(service.Network)
	p.serviceBuild.Load(projects.FormatDotEnv(service.BuildArgs))
	p.serviceEnv.Load(projects.FormatDotEnv(service.Env))
	p.serviceLabels.Load(formatStringMap(service.Labels))
	if service.Healthcheck == nil {
		p.healthForm.Field(healthFieldType).Set("none")
		return
	}
	health := service.Healthcheck
	p.healthForm.Field(healthFieldType).Set(health.Type)
	p.healthForm.Field(healthFieldScheme).Set(health.Scheme)
	p.healthForm.Field(healthFieldPath).Set(health.Path)
	if health.Port != 0 {
		p.healthForm.Field(healthFieldPort).Set(strconv.Itoa(health.Port))
	}
	if health.Interval != 0 {
		p.timingForm.Field(timingFieldInterval).Set(health.Interval.String())
	}
	if health.Timeout != 0 {
		p.timingForm.Field(timingFieldTimeout).Set(health.Timeout.String())
	}
	if health.Retries != 0 {
		p.timingForm.Field(timingFieldRetries).Set(strconv.Itoa(health.Retries))
	}
	if health.StableFor != 0 {
		p.timingForm.Field(timingFieldStableFor).Set(health.StableFor.String())
	}
}

func (p *Projects) serviceKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		p.serviceOpen = false
		return nil
	case "ctrl+s":
		p.saveService()
		return nil
	case "ctrl+t":
		p.serviceTab = (p.serviceTab + 1) % serviceTabCount
		return nil
	}

	switch p.serviceTab {
	case serviceTabBuildArgs:
		return p.serviceBuild.Update(msg)
	case serviceTabEnv:
		return p.serviceEnv.Update(msg)
	case serviceTabLabels:
		return p.serviceLabels.Update(msg)
	}
	form := p.serviceForm
	if p.serviceTab == serviceTabRuntime {
		form = p.runtimeForm
	} else if p.serviceTab == serviceTabHealth {
		form = p.healthForm
	} else if p.serviceTab == serviceTabTiming {
		form = p.timingForm
	}
	switch msg.String() {
	case "tab", "down":
		form.Next()
		return nil
	case "shift+tab", "up":
		form.Previous()
		return nil
	}
	return form.Update(msg)
}

func (p *Projects) saveService() {
	service, err := p.buildService()
	if err != nil {
		p.Fail(err)
		return
	}
	for index := range p.services {
		if index != p.serviceEditing && p.services[index].Name == service.Name {
			p.Notify("service "+service.Name+" already exists", "error")
			return
		}
	}
	if p.serviceEditing < 0 {
		p.services = append(p.services, service)
		sort.Slice(p.services, func(i, j int) bool { return p.services[i].Name < p.services[j].Name })
		for index := range p.services {
			if p.services[index].Name == service.Name {
				p.serviceCursor = index
			}
		}
	} else {
		p.services[p.serviceEditing] = service
		sort.Slice(p.services, func(i, j int) bool { return p.services[i].Name < p.services[j].Name })
	}
	p.serviceDirty = true
	p.serviceOpen = false
}

func (p *Projects) buildService() (models.Service, error) {
	name := p.serviceForm.Value(serviceFieldName)
	if !models.ValidResourceName(name) {
		return models.Service{}, fmt.Errorf("service name is invalid")
	}
	service := models.Service{
		Name: name, Dockerfile: p.serviceForm.Value(serviceFieldDockerfile), Context: p.serviceForm.Value(serviceFieldContext),
		Command: p.runtimeForm.Value(runtimeFieldCommand), Restart: p.runtimeForm.Value(runtimeFieldRestart),
		Network: p.runtimeForm.Value(runtimeFieldNetwork),
	}
	if p.serviceForm.Value(serviceFieldSource) == "image" {
		service.Image = p.serviceForm.Value(serviceFieldImage)
		if !models.ValidDigestReference(service.Image) {
			return models.Service{}, fmt.Errorf("service image must use repository@sha256:digest")
		}
	}
	if !models.ValidSourcePath(service.Dockerfile) || !models.ValidSourcePath(service.Context) {
		return models.Service{}, fmt.Errorf("service build paths must stay inside the source directory")
	}
	var err error
	service.Ports, err = parsePorts(p.runtimeForm.Value(runtimeFieldPorts))
	if err != nil {
		return models.Service{}, err
	}
	service.Volumes, err = parseVolumes(p.runtimeForm.Value(runtimeFieldVolumes))
	if err != nil {
		return models.Service{}, err
	}
	service.BuildArgs, err = projects.ParseDotEnv(p.serviceBuild.Value())
	if err != nil {
		return models.Service{}, fmt.Errorf("build args: %w", err)
	}
	service.Env, err = projects.ParseDotEnv(p.serviceEnv.Value())
	if err != nil {
		return models.Service{}, fmt.Errorf("service env: %w", err)
	}
	service.Labels, err = parseStringMap(p.serviceLabels.Value())
	if err != nil {
		return models.Service{}, fmt.Errorf("labels: %w", err)
	}
	if err := models.ValidateLabels(service.Labels); err != nil {
		return models.Service{}, err
	}
	service.Healthcheck, err = p.buildServiceHealthcheck()
	return service, err
}

func (p *Projects) buildServiceHealthcheck() (*models.Healthcheck, error) {
	typeName := p.healthForm.Value(healthFieldType)
	if typeName == "none" {
		return nil, nil
	}
	health := &models.Healthcheck{Type: typeName}
	if typeName == "running" {
		stable := p.timingForm.Value(timingFieldStableFor)
		if stable == "" {
			return nil, fmt.Errorf("healthcheck stable_for is required")
		}
		var err error
		health.StableFor, err = time.ParseDuration(stable)
		if err != nil {
			return nil, fmt.Errorf("healthcheck stable_for: %w", err)
		}
		return health, models.ValidateHealthcheck(health)
	}
	port, err := strconv.Atoi(p.healthForm.Value(healthFieldPort))
	if err != nil {
		return nil, fmt.Errorf("healthcheck port is required")
	}
	health.Port = port
	health.Scheme = p.healthForm.Value(healthFieldScheme)
	health.Path = p.healthForm.Value(healthFieldPath)
	if health.Scheme == "" {
		health.Scheme = "http"
	}
	interval := p.timingForm.Value(timingFieldInterval)
	if interval == "" {
		interval = "5s"
	}
	health.Interval, err = time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("healthcheck interval: %w", err)
	}
	timeout := p.timingForm.Value(timingFieldTimeout)
	if timeout == "" {
		timeout = "3s"
	}
	health.Timeout, err = time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("healthcheck timeout: %w", err)
	}
	retries := p.timingForm.Value(timingFieldRetries)
	if retries == "" {
		retries = "10"
	}
	health.Retries, err = strconv.Atoi(retries)
	if err != nil {
		return nil, fmt.Errorf("healthcheck retries: %w", err)
	}
	if typeName == "tcp" {
		health.Scheme = ""
		health.Path = ""
	}
	return health, models.ValidateHealthcheck(health)
}

func parseStringMap(content string) (map[string]string, error) {
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}
	values := make(map[string]string)
	if err := yaml.Unmarshal([]byte(content), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func formatStringMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	encoded, err := yaml.Marshal(values)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func (p *Projects) refreshAgentOptions() {
	agents, err := p.server.Store().ListAgents()
	if err != nil {
		p.Fail(err)
		return
	}

	builders := make([]components.Option, 0, len(agents))
	runners := make([]components.Option, 0, len(agents))
	for _, agent := range agents {
		if agent.HasRole(models.RoleBuilder) {
			builders = append(builders, components.Option{Value: agent.ID, Label: agent.Name})
		}
		if agent.HasRole(models.RoleRunner) {
			runners = append(runners, components.Option{Value: agent.ID, Label: agent.Name})
		}
	}

	p.form.Field(fieldBuilder).SetOptions(builders)
	p.form.Field(fieldRunners).SetOptions(runners)
}

func (p *Projects) load(project *models.Project) {
	kind := kindStandalone
	name := project.Name
	if project.Managed() {
		kind = kindFile
		name = project.Base()
	}

	values := map[int]string{
		fieldKind:       kind,
		fieldName:       name,
		fieldEnvName:    project.Env,
		fieldGitURL:     project.GitURL,
		fieldBranch:     project.Branch,
		fieldDockerfile: project.Dockerfile,
		fieldContext:    project.Context,
		fieldBuilder:    project.Builder,
		fieldPorts:      formatPorts(project.Runtime.Ports),
		fieldVolumes:    formatVolumes(project.Runtime.Volumes),
		fieldNetwork:    project.Runtime.Network,
		fieldAuto:       formatBool(project.AutoDeploy),
	}
	for index, value := range values {
		p.form.Fields[index].Set(value)
	}
	p.form.Field(fieldRunners).SetValues(project.Runners)
	p.form.Cursor = 0
}

func (p *Projects) save() {
	if missing := p.required(); missing != "" {
		p.Notify(missing+" is required", "error")
		p.tab = tabSettings
		return
	}

	variables, err := projects.ParseDotEnv(p.variables.Value())
	if err != nil {
		p.Fail(fmt.Errorf(".env: %w", err))
		p.tab = tabVariables
		return
	}

	if p.fileMode() {
		if p.serviceDirty && p.yaml.Dirty() {
			p.Notify("save either structured services or raw config changes, not both at once", "error")
			return
		}
		p.saveToFiles()
		return
	}
	p.saveStandalone(variables)
}

func (p *Projects) required() string {
	if p.fileMode() && p.yaml.Dirty() {
		for _, index := range []int{fieldName, fieldEnvName, fieldGitURL} {
			if p.form.Value(index) == "" {
				return p.form.Field(index).Label
			}
		}
		return ""
	}
	return p.form.Missing()
}

func (p *Projects) saveStandalone(variables map[string]string) {
	project, err := p.buildProject(variables)
	if err != nil {
		p.Fail(err)
		return
	}

	if p.editing != "" && p.editing != project.Name {
		if err := p.server.Store().DeleteProject(p.editing); err != nil {
			p.Fail(err)
			return
		}
	}
	if err := p.server.Store().SaveProject(project); err != nil {
		p.Fail(err)
		return
	}

	p.finish("saved " + project.Name)
}

func (p *Projects) saveToFiles() {
	ports, volumes, err := p.runtimeSpec()
	if err != nil {
		p.Fail(err)
		return
	}

	envName := p.form.Value(fieldEnvName)
	if envName == "" {
		envName = defaultEnvName
	}

	autoDeploy := p.form.Value(fieldAuto) == "yes"
	draft := projects.Draft{
		Project: p.form.Value(fieldName),
		Env:     envName,
		Definition: projects.Definition{
			Git:        p.form.Value(fieldGitURL),
			Dockerfile: p.form.Value(fieldDockerfile),
			Context:    p.form.Value(fieldContext),
		},
		Environment: projects.Environment{
			Branch:     p.form.Value(fieldBranch),
			Builder:    p.agentName(p.form.Value(fieldBuilder)),
			Runners:    p.agentNames(p.form.Values(fieldRunners)),
			AutoDeploy: &autoDeploy,
			Ports:      models.FormatPorts(ports),
			Volumes:    models.FormatVolumes(volumes),
			Network:    p.form.Value(fieldNetwork),
		},
		RawEnv: p.variables.Value(),
	}
	if p.serviceDirty {
		draft.Environment.Services = serviceDeclarations(p.services)
	}
	if p.yaml.Dirty() {
		if err := validateYAML(p.yaml.Value()); err != nil {
			p.Fail(err)
			p.tab = tabYAML
			return
		}
		draft.RawYAML = p.yaml.Value()
	}

	if err := p.server.Loader().Write(draft); err != nil {
		p.Fail(err)
		return
	}

	loaded := p.server.ReloadProjects()
	p.finish(fmt.Sprintf("wrote %s/%s and reloaded %d project(s)", draft.Project, envName, loaded))
}

func (p *Projects) finish(message string) {
	p.variables.MarkSaved()
	p.yaml.MarkSaved()
	p.serviceDirty = false
	p.mode = projectList
	p.reload()
	p.Notify(message, "success")
}

func (p *Projects) runtimeSpec() ([]models.Port, []models.Volume, error) {
	ports, err := parsePorts(p.form.Value(fieldPorts))
	if err != nil {
		return nil, nil, err
	}
	volumes, err := parseVolumes(p.form.Value(fieldVolumes))
	if err != nil {
		return nil, nil, err
	}
	return ports, volumes, nil
}

func (p *Projects) buildProject(variables map[string]string) (*models.Project, error) {
	ports, volumes, err := p.runtimeSpec()
	if err != nil {
		return nil, err
	}

	return &models.Project{
		Name:       p.form.Value(fieldName),
		GitURL:     p.form.Value(fieldGitURL),
		Branch:     p.form.Value(fieldBranch),
		Dockerfile: p.form.Value(fieldDockerfile),
		Context:    p.form.Value(fieldContext),
		Builder:    p.form.Value(fieldBuilder),
		Runners:    p.form.Values(fieldRunners),
		AutoDeploy: p.form.Value(fieldAuto) == "yes",
		Services:   append([]models.Service(nil), p.services...),
		Runtime: models.Runtime{
			Ports:   ports,
			Volumes: volumes,
			Network: p.form.Value(fieldNetwork),
			Env:     variables,
		},
	}, nil
}

func serviceDeclarations(services []models.Service) map[string]projects.Service {
	declarations := make(map[string]projects.Service, len(services))
	for _, service := range services {
		declaration := projects.Service{
			Image: service.Image, Dockerfile: service.Dockerfile, Context: service.Context,
			BuildArgs: service.BuildArgs, Command: service.Command,
			Ports: models.FormatPorts(service.Ports), Volumes: models.FormatVolumes(service.Volumes),
			Env: service.Env, Network: service.Network, Restart: service.Restart, Labels: service.Labels,
		}
		if service.Healthcheck != nil {
			health := service.Healthcheck
			declaration.Healthcheck = &projects.Healthcheck{
				Type: health.Type, Scheme: health.Scheme, Path: health.Path, Port: health.Port,
			}
			if health.Interval != 0 {
				declaration.Healthcheck.Interval = health.Interval.String()
			}
			if health.Timeout != 0 {
				declaration.Healthcheck.Timeout = health.Timeout.String()
			}
			if health.Retries != 0 {
				retries := health.Retries
				declaration.Healthcheck.Retries = &retries
			}
			if health.StableFor != 0 {
				declaration.Healthcheck.StableFor = health.StableFor.String()
			}
		}
		declarations[service.Name] = declaration
	}
	return declarations
}

func (p *Projects) confirm(msg tea.KeyMsg, action func()) tea.Cmd {
	switch msg.String() {
	case "y":
		action()
		p.mode = projectList
		p.reload()
	case "n", "esc":
		p.mode = projectList
	}
	return nil
}

func (p *Projects) deploy() {
	project := p.selected()
	if project == nil {
		return
	}

	release, err := p.server.Pipeline().Trigger(project.Name, "", models.TriggerManual)
	if err != nil {
		p.Fail(err)
		return
	}
	p.Notify("release "+release.ID+" building on "+release.BuilderName, "success")
}

func (p *Projects) rollback() {
	project := p.selected()
	if project == nil {
		return
	}

	release, err := p.server.Pipeline().Rollback(project.Name, "")
	if err != nil {
		p.Fail(err)
		return
	}
	p.Notify("rolling "+project.Name+" back to "+theme.ImageTag(release.Image), "success")
}

func (p *Projects) stop() {
	project := p.selected()
	if project == nil {
		return
	}

	if err := p.server.Pipeline().Stop(project.Name); err != nil {
		p.Fail(err)
		return
	}
	p.Notify(project.Name+" stopped on every runner", "success")
}

func (p *Projects) remove() {
	project := p.selected()
	if project == nil {
		return
	}

	if err := p.server.Pipeline().Remove(project.Name); err != nil {
		p.Fail(err)
		return
	}
	if project.Managed() {
		if err := p.server.Loader().Remove(project.Base(), project.Env); err != nil {
			p.Fail(err)
		}
	}
	p.Fail(p.server.Store().DeleteProject(project.Name))
}

func (p *Projects) agentName(id string) string {
	if id == "" {
		return ""
	}
	if agent, err := p.server.Store().GetAgent(id); err == nil {
		return agent.Name
	}
	return id
}

func (p *Projects) agentNames(ids []string) []string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, p.agentName(id))
	}
	return names
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func validateYAML(content string) error {
	if err := projects.ValidateEnvironmentYAML(content); err != nil {
		return fmt.Errorf("invalid yaml: %w", err)
	}
	return nil
}

func (p *Projects) Hints() []components.Hint {
	switch p.mode {
	case projectEdit:
		if p.serviceOpen {
			return []components.Hint{{Key: "ctrl+t", Label: "next service tab"}, {Key: "tab", Label: "next field"}, {Key: "ctrl+s", Label: "save service"}, {Key: "esc", Label: "cancel"}}
		}
		hints := []components.Hint{{Key: "ctrl+t", Label: "next tab"}}
		if p.tab == tabSettings {
			hints = append(hints, components.Hint{Key: "tab", Label: "next field"})
		} else if p.tab == tabServices {
			hints = append(hints, components.Hint{Key: "n", Label: "add"}, components.Hint{Key: "e / enter", Label: "edit"}, components.Hint{Key: "d", Label: "remove"})
		}
		return append(hints,
			components.Hint{Key: "ctrl+s", Label: "save"},
			components.Hint{Key: "esc", Label: "cancel"})
	case projectConfirmDelete, projectConfirmDeploy, projectConfirmRollback, projectConfirmStop:
		return []components.Hint{{Key: "y", Label: "confirm"}, {Key: "n", Label: "cancel"}}
	default:
		return []components.Hint{
			{Key: "↑↓", Label: "select"},
			{Key: "enter", Label: "deploy"},
			{Key: "n", Label: "new"},
			{Key: "e", Label: "edit"},
			{Key: "r", Label: "rollback"},
			{Key: "s", Label: "stop"},
			{Key: "ctrl+t", Label: "overview / variables / config"},
			{Key: "R", Label: "reload files"},
			{Key: "d", Label: "delete"},
		}
	}
}

func (p *Projects) Render() string {
	switch p.mode {
	case projectEdit:
		return p.renderEdit()
	case projectConfirmDelete:
		return p.renderConfirm("Delete project", "Delete "+p.name()+" and remove its containers?", true, "delete")
	case projectConfirmDeploy:
		return p.renderDeploy()
	case projectConfirmRollback:
		return p.renderConfirm("Roll back", "Roll "+p.name()+" back to its last successful image?", false, "roll back")
	case projectConfirmStop:
		return p.renderConfirm("Stop project", "Stop "+p.name()+" on all runners?", true, "stop")
	default:
		return p.renderList()
	}
}

func (p *Projects) name() string {
	if project := p.selected(); project != nil {
		return project.Name
	}
	return ""
}

func (p *Projects) renderEdit() string {
	title := "new project"
	if p.editing != "" {
		title = "edit " + p.editing
	}

	note := "stored in uruflow"
	if p.fileMode() {
		note = p.server.ProjectsDir() + "/" + p.form.Value(fieldName) + "/"
	}

	body := components.TabBar(p.tabs(), p.tab, components.CardWidth(p.Width), note) + "\n"

	switch p.tab {
	case tabVariables:
		body += theme.Ghost.Render("environment variables — paste a .env file or type KEY=VALUE lines") +
			"\n" + p.variables.Render()
	case tabServices:
		if p.serviceOpen {
			body += p.renderServiceEditor()
		} else {
			body += p.renderServices()
		}
	case tabYAML:
		body += theme.Ghost.Render("paste "+p.form.Value(fieldEnvName)+".yaml here and it is written as the file") +
			"\n" + p.yaml.Render()
	default:
		body += p.form.Render(components.CardWidth(p.Width))
	}

	return components.Stack(
		components.Card(title, body, p.Width, true),
		p.Notice(),
	)
}

func (p *Projects) renderServices() string {
	if len(p.services) == 0 {
		return theme.Ghost.Render("SERVICES\n\n  no explicit services — the project uses its standalone runtime\n  press n to add a service")
	}
	lines := []string{theme.Heading.Render("SERVICES"), ""}
	for index, service := range p.services {
		pointer := "  "
		if index == p.serviceCursor {
			pointer = theme.Lead.Render(theme.IconPointer + " ")
		}
		mode := theme.Mark.Render("built")
		source := service.BuildFile()
		if !service.Built() {
			mode = theme.Note.Render("image")
			source = theme.Truncate(service.Image, 42)
		}
		lines = append(lines, pointer+theme.Body.Render(theme.Cell(service.Name, 18))+theme.Cell(mode, 12)+theme.Ghost.Render(source))
	}
	return strings.Join(lines, "\n")
}

func (p *Projects) renderServiceEditor() string {
	items := []components.TabItem{
		{Label: "settings"}, {Label: "runtime"}, {Label: "health"}, {Label: "timing"},
		{Label: "build args", Dirty: p.serviceBuild.Dirty()},
		{Label: "env", Dirty: p.serviceEnv.Dirty()},
		{Label: "labels", Dirty: p.serviceLabels.Dirty()},
	}
	body := components.TabBar(items, p.serviceTab, components.CardWidth(p.Width), "service") + "\n"
	switch p.serviceTab {
	case serviceTabRuntime:
		body += p.runtimeForm.Render(components.CardWidth(p.Width))
	case serviceTabHealth:
		body += p.healthForm.Render(components.CardWidth(p.Width))
	case serviceTabTiming:
		body += p.timingForm.Render(components.CardWidth(p.Width))
	case serviceTabBuildArgs:
		body += theme.Ghost.Render("build arguments — KEY=VALUE lines") + "\n" + p.serviceBuild.Render()
	case serviceTabEnv:
		body += theme.Ghost.Render("service environment — KEY=VALUE lines") + "\n" + p.serviceEnv.Render()
	case serviceTabLabels:
		body += theme.Ghost.Render("docker labels — YAML string map; uruflow.* is reserved") + "\n" + p.serviceLabels.Render()
	default:
		body += p.serviceForm.Render(components.CardWidth(p.Width))
	}
	return body
}

func (p *Projects) renderList() string {
	table := components.Table{
		Columns: []components.Column{
			{Title: "project", Width: 16},
			{Title: "env", Width: 8},
			{Title: "from", Width: 7},
			{Title: "repository", Width: 22, Flex: true},
			{Title: "branch", Width: 12},
			{Title: "builder", Width: 14},
			{Title: "svc", Width: 4, Right: true},
			{Title: "runners", Width: 8, Right: true},
			{Title: "auto", Width: 6},
			{Title: "last release", Width: 26},
			{Title: "when", Width: 10, Right: true},
		},
		Cursor: p.cursor,
		Height: p.TableHeight(16),
		Empty:  "no projects yet — press n to add one",
	}

	for index := range p.projects {
		project := &p.projects[index]

		auto := components.Styled("off", theme.Ghost)
		if project.AutoDeploy {
			auto = components.Styled("on", theme.Good)
		}

		source := components.Styled("tui", theme.Ghost)
		if project.Managed() {
			source = components.Styled("file", theme.Note)
		}

		environment := components.Styled("–", theme.Ghost)
		label := project.Name
		if project.Env != "" {
			environment = components.Styled(project.Env, theme.Mark)
			label = project.Base()
		}

		status := components.Styled("never deployed", theme.Ghost)
		when := components.Styled("–", theme.Ghost)
		if release, found := p.last[project.Name]; found {
			status = components.Text(components.Steps(components.ReleaseSteps(&release)))
			when = components.Styled(theme.Since(release.StartedAt), theme.Ghost)
		}

		table.Rows = append(table.Rows, components.Row{
			components.Text(label),
			environment,
			source,
			components.Styled(project.GitURL, theme.Ghost),
			components.Styled(project.Branch, theme.Note),
			components.Styled(p.agentName(project.Builder), theme.Mark),
			components.Styled(fmt.Sprint(len(project.ServiceList())), theme.Body),
			components.Styled(fmt.Sprint(len(project.Runners)), theme.Body),
			auto,
			status,
			when,
		})
	}

	body := components.Card("projects", table.Render(components.CardWidth(p.Width)), p.Width, false)

	if project := p.selected(); project != nil {
		body = components.Stack(body,
			components.Card(project.Name, p.detailPanel(project), p.Width, true))
	}

	return components.Stack(body, p.problems(), p.Notice())
}

func (p *Projects) problems() string {
	found := p.server.ProjectProblems()
	if len(found) == 0 {
		return ""
	}

	lines := make([]string, 0, len(found))
	for _, problem := range found {
		lines = append(lines, theme.Bad.Render(theme.IconFailure+" ")+
			theme.Ghost.Render(theme.Truncate(problem.Path, 48))+"  "+
			theme.Body.Render(problem.Reason.Error()))
	}

	return components.Card("file problems", strings.Join(lines, "\n"), p.Width, false)
}

func (p *Projects) detailPanel(project *models.Project) string {
	items := []components.TabItem{
		{Label: "overview"},
		{Label: "variables"},
		{Label: "config"},
	}

	note := "stored in uruflow"
	if project.Managed() {
		note = project.Source
	}

	body := components.TabBar(items, p.detailTab, components.CardWidth(p.Width), note) + "\n"

	switch p.detailTab {
	case detailVariables:
		body += p.detailVariables(project)
	case detailConfig:
		body += p.detailConfig(project)
	default:
		body += p.detail(project)
	}

	return body
}

func (p *Projects) detailVariables(project *models.Project) string {
	if len(project.Runtime.Env) == 0 {
		return theme.Ghost.Render("no environment variables — press e to add some")
	}

	content := projects.FormatDotEnv(project.Runtime.Env)
	lines := make([]string, 0, len(project.Runtime.Env))
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		key, value, _ := strings.Cut(line, "=")
		lines = append(lines, theme.Mark.Render(theme.Cell(key, 26))+theme.Body.Render(value))
	}

	return strings.Join(lines, "\n")
}

func (p *Projects) detailConfig(project *models.Project) string {
	if !project.Managed() {
		return theme.Ghost.Render("standalone project — it has no file; press e to edit it")
	}

	content, err := readFile(project.Source)
	if err != nil {
		return theme.Bad.Render("cannot read " + project.Source + ": " + err.Error())
	}
	if strings.TrimSpace(content) == "" {
		return theme.Bad.Render("cannot read " + project.Source)
	}

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for index, line := range lines {
		lines[index] = theme.Body.Render(line)
	}
	return strings.Join(lines, "\n")
}

func (p *Projects) detail(project *models.Project) string {
	lines := []string{}
	if project.Managed() {
		lines = append(lines, components.KeyValue("defined in", theme.Note.Render(project.Source), 12))
	}
	lines = append(lines,
		components.KeyValue("image", theme.Note.Render(p.server.Registry().ImageRepository(project.Name)), 12),
		components.KeyValue("build", project.BuildFile()+theme.Ghost.Render("  context ")+project.BuildContext(), 12),
		components.KeyValue("runners", strings.Join(p.agentNames(project.Runners), ", "), 12),
	)

	if ports := formatPorts(project.Runtime.Ports); ports != "" {
		lines = append(lines, components.KeyValue("ports", ports, 12))
	}
	if volumes := formatVolumes(project.Runtime.Volumes); volumes != "" {
		lines = append(lines, components.KeyValue("volumes", volumes, 12))
	}
	if len(project.Runtime.Env) > 0 {
		lines = append(lines, components.KeyValue("env",
			fmt.Sprintf("%d variables", len(project.Runtime.Env)), 12))
	}

	lines = append(lines, "")
	lines = append(lines, p.serviceDetails(project)...)

	return strings.Join(lines, "\n")
}

func (p *Projects) serviceDetails(project *models.Project) []string {
	lines := []string{theme.Heading.Render("SERVICES")}
	for _, service := range project.ServiceList() {
		name := service.Name
		if name == "" {
			name = "default"
		}
		mode := "built"
		source := service.BuildFile()
		if !service.Built() {
			mode = "image"
			source = theme.Truncate(service.Image, 32)
		}
		health := "none"
		if service.Healthcheck != nil {
			health = service.Healthcheck.Type
		}
		ports := formatPorts(service.Ports)
		if ports == "" {
			ports = "–"
		}
		network := service.Network
		if network == "" {
			network = "default"
		}
		if p.Width < 120 {
			lines = append(lines,
				"  "+theme.Body.Render(theme.Cell(name, 14))+theme.Mark.Render(theme.Cell(mode, 8))+theme.Ghost.Render(theme.Truncate(source, 44)),
				theme.Ghost.Render("    ports "+theme.Cell(ports, 14)+" net "+theme.Cell(network, 10)+" health "+theme.Cell(health, 9)+fmt.Sprintf(" labels %d", len(service.Labels))))
			continue
		}
		lines = append(lines, "  "+theme.Body.Render(theme.Cell(name, 14))+theme.Mark.Render(theme.Cell(mode, 8))+
			theme.Ghost.Render(theme.Cell(source, 30)+" ports "+theme.Cell(ports, 16)+" net "+theme.Cell(network, 12)+" health "+theme.Cell(health, 9)+fmt.Sprintf(" labels %d", len(service.Labels))))
	}
	return lines
}

func (p *Projects) renderDeploy() string {
	project := p.selected()
	if project == nil {
		return ""
	}

	dialog := components.Dialog{
		Title: "Deploy " + project.Name,
		Message: "Build on " + p.agentName(project.Builder) + ", then release to " +
			fmt.Sprintf("%d runner(s)", len(project.Runners)) + ".",
		Detail:  p.server.Registry().ImageRepository(project.Name),
		Confirm: "deploy",
	}
	return dialog.Render(p.Width, p.Height)
}

func (p *Projects) renderConfirm(title, message string, danger bool, confirm string) string {
	dialog := components.Dialog{
		Title:   title,
		Message: message,
		Confirm: confirm,
		Danger:  danger,
	}
	return dialog.Render(p.Width, p.Height)
}
