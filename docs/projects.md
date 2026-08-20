# Projects

A project is one repository deployed to one set of runners. This page covers how to define one, how
environments work, and how environment variables resolve.

## 1. Two Ways to Define a Project

| | Standalone | File-backed |
| :--- | :--- | :--- |
| Stored in | The database | `projects/<name>/` plus the database |
| Created by | The terminal interface | The interface, or by writing files |
| Version control | No | Yes |
| Best for | A single deployment, quick experiments | Several environments, review, reproducibility |

Both produce identical projects. The interface marks which is which, and can create either — you
never have to write files by hand.

## 2. Creating a Project in the Interface

Press `2`, then `n`. The first field chooses where it lives (all keys are listed in
[Terminal Interface](interface.md)):

```text
▸ stored as     ○ standalone  ◉ file       ‹ › to change
```

The form has three tabs, cycled with `ctrl+t`:

| Tab | Contents |
| :--- | :--- |
| `settings` | Name, git URL, branch, Dockerfile, context, ports, volumes, network, and pickers for builder, runners and auto-deploy |
| `variables` | The `.env` file — paste one in, or type `KEY=VALUE` lines |
| `config` | File mode only. Paste `<env>.yaml` to use it instead of the settings tab. |

`ctrl+s` validates before writing anything. YAML must parse; the `.env` must parse, with the offending
line reported. You are moved to the tab that failed and nothing touches disk.

Agent names are never typed. `builder` is a radio list and `runners` are checkboxes, both drawn from
agents that actually hold the role, so a project cannot reference an agent that does not exist.

When the `config` tab has pasted content, only **name**, **environment** and **git url** are required
from the settings tab — branch, builder, runners and the rest come from your YAML.

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
git: git@github.com:acme/api.git     # required
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
  worker:
    dockerfile: Dockerfile.worker
    command: ./worker
  cache:
    image: redis:7-alpine
    volumes: ["/srv/shop/redis:/data"]
```

Each service becomes its own container:

```text
uruflow-shop-prod-app
uruflow-shop-prod-worker
uruflow-shop-prod-cache
```

A service is **built** when it declares `dockerfile`, and **prebuilt** when it declares `image`.
Declaring both is rejected. Built services get their own image repository,
`<registry>/<namespace>/<project>-<service>`, so each is versioned independently by commit.

Every field a single-service project supports is available per service: `command`, `ports`,
`volumes`, `env`, `network`, `restart` and `build_args`.

### Service Variables

Service `env` merges on top of the project's:

```text
defaults.yaml → project.yaml → <env>.yaml env → <env>.env → service env
```

A variable set on one service is invisible to the others.

### All or Nothing

Services are replaced in order, and **if any service fails to become ready, every service already
replaced in that release is restored**. A release either moves the whole project forward on a runner
or leaves it exactly as it was. See [Deployments](deployments.md#3-release-safety).

### Single-Service Projects Are Unchanged

Omitting `services` keeps the existing behaviour exactly: one container named `uruflow-<project>`,
one image repository, and the project-level `dockerfile`, `context`, `ports`, `volumes` and `command`
fields apply to it. Nothing you have today needs changing.

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

To read a project's effective variables, select it and press `ctrl+t` to cycle the detail panel to
`variables`.

### Secrets

Values that must not be readable in a file or on screen are stored separately and referenced:

Press `7` in the interface, then `n`, and store the value under a name such as `api_db_url`. The
value is masked while you type and never displayed again.

```ini
# projects/api/prod.env — safe to commit
DATABASE_URL=${secret:api_db_url}
LOG_LEVEL=info
```

URUFLOW resolves the reference when a release is dispatched. The value is encrypted at rest, never
written to a project file, never shown in the interface, and never appears in a release log. A
reference to a secret that does not exist fails the deploy **before** any build starts.

Secrets live in the `7` view, where `n` stores one and `d` removes one.

> Variables that are not secret references are stored in plaintext in `.env` files and in the
> database, and are visible in the interface. See [Security](security.md#7-secrets).

## 7. Editing Safely

A file-backed project can be edited in the interface. The `settings` tab owns git URL, Dockerfile,
context, branch, builder, runners, ports, volumes, network and auto-deploy.

Everything else in those files is **preserved** on save, including `build_args`, `command`, `restart`
and project-level `env`. Those fields have no form control and can only be set by editing the files —
either in the `config` tab or on disk.

Comments in YAML files are lost when the settings tab writes them back. Content pasted into the
`config` tab is written verbatim, so comments survive that path.

## 8. Reloading

Press `R` in the projects view to re-read `projects/` from disk. Files edited outside URUFLOW are not
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
