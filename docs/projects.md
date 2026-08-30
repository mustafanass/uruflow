# Projects

For the complete production schema—multi-repository services, managed Docker resources, dependency
graphs, jobs, structured commands, security/resource controls and command probes—see
[Native Build Model](native-build-model.md).

A project is one release unit deployed to one set of runners. It has a primary repository and may
override the repository and branch for individual services. This page covers how to define one, how
environments work, and how environment variables resolve.

## 1. One Authoritative Definition

Every project is defined by version-controlled YAML under `projects/<name>/`. The database holds the
loaded effective model and runtime history, but it is never a competing configuration source.

Create files with any editor, `project edit` in the operations workspace, configuration management,
or inline YAML input in the same page. Validate and reload them from its prompt.

## 2. Creating and Operating a Project

The stage is explicit and derived from build targets and release targets:

| Workflow | Builder | Runners | Result |
| :--- | :---: | :---: | :--- |
| `build_only` | Required | Not used | Build and publish immutable artifacts |
| `deploy_only` | Not used | Required | Release already-pinned images |
| `build_deploy` | Required | Required | Build, publish, then release together |

```text
project validate projects/api/prod.yaml
project reload
project show api-prod
project deploy api-prod
```

The loader resolves builder and runner names against enrolled agents and verifies their roles.
Branch, workflow, services, variables and runtime policy always come from the files.

## 3. File Layout

```text
/etc/uruflow/
  config.yaml              Server, registry and webhook settings
  defaults.yaml            Environment variables shared by every project
  projects/
    api/
      project.yaml         Shared by every environment of this project
      dev.yaml             → project api-dev
      dev.env              Variables for api-dev
      prod.yaml            → project api-prod
      prod.env             Variables for api-prod
```

The directory name is a convention. The project name comes from `project.yaml`, or from the directory
if that field is absent. Each `<env>.yaml` produces a project named `<project>-<env>`.

### project.yaml

What every environment shares.

```yaml
name: api                            # optional; defaults to the directory name
git: git@github.com:acme/api.git     # required by build workflows
dockerfile: Dockerfile               # optional; defaults to Dockerfile
context: .                           # optional; defaults to .
build_args:                          # optional; passed to docker build
  VERSION: "2"
env:                                 # optional; variables for every environment
  APP_NAME: api
```

### `<env>.yaml`

What differs per environment.

```yaml
branch: develop                      # required
builder: builder-01                  # required; agent name, must hold the builder role
runners: [web-01, web-02]            # required; agent names, must hold the runner role
workflow: build_deploy                # build_deploy, build_only, or deploy_only
auto_deploy: true                    # optional; defaults to true
ports: ["8081:80"]                   # optional; host:container[/protocol]
volumes: ["/srv/api:/data:ro"]       # optional; source:target[:ro]
network: uruflow-net                 # optional; docker network name
restart: unless-stopped              # optional; defaults to unless-stopped
command: ""                          # optional; overrides the image command
env:                                 # optional; variables for this environment
  MODE: production
```

Unknown keys are rejected by name, so a typo such as `brnach:` is reported as a typo rather than
surfacing later as a missing branch.

### `<env>.env`

An ordinary dotenv file. Comments, `export` prefixes, single and double quotes, and escaped newlines
are all supported. Malformed lines are rejected with a line number.

```ini
# api dev
LOG_LEVEL=debug
DATABASE_URL=postgres://dev-host/api
GREETING="hello world"
```

## 4. Multiple Services

A project runs one container per runner by default. Add a `services` block to run several — an
application and a worker built from the same repository, plus a prebuilt dependency:

```yaml
# projects/shop/prod.yaml
branch: main
builder: builder-01
runners: [web-01]

services:
  app:
    dockerfile: Dockerfile
    context: .
    ports: ["8080:80"]
    env:
      ROLE: web
    healthcheck:
      type: http
      path: /health
      port: 80
      interval: 5s
      timeout: 3s
      retries: 10
    labels:
      traefik.enable: "true"
  worker:
    dockerfile: Dockerfile.worker
    command: ./worker
  cache:
    image: redis@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
    volumes: ["/srv/shop/redis:/data"]
```

Each service becomes its own container:

```text
uruflow-shop-prod-app
uruflow-shop-prod-worker
uruflow-shop-prod-cache
```

A service is **built** when it declares `dockerfile`, and **prebuilt** when it declares `image`.
Declaring both is rejected. Prebuilt images must use an immutable `repository@sha256:digest`
reference; mutable tags are rejected. Built services get their own image repository,
`<registry>/<namespace>/<project>-<service>`, so each is versioned independently by commit.

Every field a single-service project supports is available per service. Multi-service projects also
support source overrides, dependency conditions, jobs, exact commands, typed mounts, multiple
networks, resource/security/logging controls, native readiness, and generic Docker labels. See the
[Native Build Model](native-build-model.md) for the full schema.

### Service Healthchecks

Four readiness policies are supported:

```yaml
services:
  api:
    healthcheck:
      type: http
      scheme: http       # optional; defaults to http
      path: /health      # required
      port: 8080         # required; container port
      interval: 5s       # optional; defaults to 5s
      timeout: 3s        # optional; defaults to 3s per attempt
      retries: 10        # optional; defaults to 10 attempts
  cache:
    healthcheck:
      type: tcp
      port: 6379
  worker:
    healthcheck:
      type: running
      stable_for: 10s
  database:
    healthcheck:
      type: command
      command: [pg_isready]
      interval: 5s
      timeout: 3s
      retries: 10
```

HTTP accepts only `2xx`. HTTP and TCP require a valid port and use finite attempts. `command` becomes
a Docker `CMD` or `CMD-SHELL` healthcheck. `running` requires the container to remain running without
a restart for `stable_for`. Probe checks accept `start_period`; durations must be positive. Unknown
types, fields and malformed paths are rejected with the service field path.

See [Deployments](deployments.md#readiness) for runtime precedence and failure behavior.

### Docker Labels

Labels are a string map and are copied unchanged to both built and prebuilt service containers:

```yaml
services:
  api:
    dockerfile: Dockerfile
    labels:
      traefik.enable: "true"
      traefik.http.routers.api.rule: "Host(`api.example.com`)"
      traefik.http.services.api.loadbalancer.server.port: "8080"
```

Traefik is only an example consumer; it is not built into or required by URUFLOW. Caddy, monitoring
agents and other Docker-integrated tools can consume the same generic labels. Keys beginning with
`uruflow.` are reserved for ownership and release identity and are rejected in project configuration.

### Service Variables

Service `env` merges on top of the project's:

```text
defaults.yaml → project.yaml → <env>.yaml env → <env>.env → service env
```

A variable set on one service is invisible to the others.

### All or Nothing

Services are replaced in dependency order, and **if any service fails to become ready, every
long-running service already replaced in that release is restored**. Completed job side effects
cannot be reversed, so migrations must be backward-compatible. See
[Deployments](deployments.md#3-release-safety).

### Single-Service Projects Are Unchanged

Omitting `services` keeps the existing behaviour exactly: one container named `uruflow-<project>`,
one image repository, and the project-level `dockerfile`, `context`, `ports`, `volumes` and `command`
fields apply to it.

Project, environment and service names use lowercase letters, digits, `.`, `_` and `-`, start with a
letter or digit, and are at most 63 characters. Dockerfile and build-context paths must stay inside
the checked-out source directory.

## 5. Environments

`projects/api/dev.yaml` and `projects/api/prod.yaml` expand into two ordinary projects:

| | `api-dev` | `api-prod` |
| :--- | :--- | :--- |
| Branch | `develop` | `main` |
| Runners | `dev-01` | `web-01`, `web-02` |
| Container | `uruflow-api-dev` | `uruflow-api-prod` |
| Image repository | `…/uruflow/api-dev` | `…/uruflow/api-prod` |
| Release history | Separate | Separate |

**URUFLOW has no environment type.** Environments exist in the file format to remove duplication and
are expanded before the pipeline, the runner, the registry or the schema see anything. This keeps the
deployment model flat and is why adding environments changed nothing downstream.

Two consequences worth knowing:

- Because each project has its own image repository, `api-dev:latest` and `api-prod:latest` are
  unrelated tags.
- Promoting the exact image from `api-dev` to `api-prod` is therefore not possible today; `api-prod`
  builds from its own branch.

## 6. Environment Variables

Variables merge in one direction. Later wins.

```text
defaults.yaml  →  project.yaml env  →  <env>.yaml env  →  <env>.env
```

`defaults.yaml` is the shared file for every project:

```yaml
env:
  TZ: Asia/Baghdad
  LOG_LEVEL: info
```

With the `dev.env` above, `api-dev` receives:

| Variable | Value | From |
| :--- | :--- | :--- |
| `TZ` | `Asia/Baghdad` | `defaults.yaml` |
| `APP_NAME` | `api` | `project.yaml` |
| `MODE` | `production` | `<env>.yaml` — only where set |
| `LOG_LEVEL` | `debug` | `dev.env`, overriding `defaults.yaml` |
| `DATABASE_URL` | `postgres://dev-host/api` | `dev.env` |

`defaults.yaml` is not created by `uruflow init`. Create it yourself next to `config.yaml` when you
want shared variables.

Use `project show <name>` in the workspace to inspect the loaded effective project.

### Secrets

Values that must not be readable in a file or on screen are stored separately and referenced:

Store a value from the masked workspace prompt with `secret set api_db_url`. It is never displayed again.

```ini
# projects/api/prod.env — safe to commit
DATABASE_URL=${secret:api_db_url}
LOG_LEVEL=info
```

URUFLOW resolves the reference when a release is dispatched. The value is encrypted at rest, never
written to a project file, never shown in the interface, and never appears in a release log. A
reference to a secret that does not exist fails the deploy **before** any build starts. Use
Use `secret list` and `secret remove <name>` in the workspace to manage stored names.

> Variables that are not secret references are stored in plaintext in `.env` files and in the
> database, and are visible in the interface. See [Security](security.md#7-secrets).

## 7. Editing Safely

`project edit <name>` in the workspace opens the authoritative environment YAML in `$VISUAL` or
`$EDITOR` and reloads after the editor closes. It also accepts `project apply <project> <env> -`; paste
YAML and press `Ctrl+S` to validate and save it atomically. Failed full validation restores the
previous file, so invalid desired state is not left behind.

## 8. Reloading

Run `project reload` in the workspace to re-read `projects/` from disk. Files edited outside URUFLOW are not
noticed until you do.

Loading never deletes. If a file disappears, the project keeps running and is reported as orphaned:

```text
╭ FILE PROBLEMS ────────────────────────────────────────────────╮
│ ✘ projects/api/dev.yaml   api-dev came from this file, which  │
│                           is gone — press d to remove it      │
╰───────────────────────────────────────────────────────────────╯
```

A file that fails to load never takes the others down. Each problem names the file and the reason.

## 9. Complete Example

Two environments of one service, sharing a repository and a set of variables.

```yaml
# /etc/uruflow/defaults.yaml
env:
  TZ: Asia/Baghdad
  LOG_LEVEL: info
```

```yaml
# /etc/uruflow/projects/api/project.yaml
git: git@github.com:acme/api.git
dockerfile: Dockerfile
context: .
build_args:
  VERSION: "2"
env:
  APP_NAME: api
```

```yaml
# /etc/uruflow/projects/api/dev.yaml
branch: develop
builder: builder-01
runners: [dev-01]
auto_deploy: true
ports: ["8081:80"]
```

```ini
# /etc/uruflow/projects/api/dev.env
LOG_LEVEL=debug
DATABASE_URL=postgres://dev-host/api
```

```yaml
# /etc/uruflow/projects/api/prod.yaml
branch: main
builder: builder-01
runners: [web-01, web-02]
auto_deploy: false
ports: ["80:80"]
volumes: ["/srv/api:/data:ro"]
restart: unless-stopped
env:
  MODE: production
```

```ini
# /etc/uruflow/projects/api/prod.env
DATABASE_URL=postgres://prod-host/api
```

A push to `develop` deploys `api-dev` automatically. A push to `main` does nothing, because
`api-prod` sets `auto_deploy: false` — production is released deliberately from the interface.

For how a push is matched to a project, see [Configuration](configuration.md#6-webhooks).
