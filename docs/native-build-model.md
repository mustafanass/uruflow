# Native Build Model

URUFLOW projects describe a reproducible build graph and the runtime state a runner must converge to.
The model is intentionally smaller than Docker Compose, but it carries the production controls needed
to build several repositories, prepare Docker resources, run initialization jobs, and release a
multi-service application as one unit.

## 1. Design Rules

1. A project selects exactly one builder and one or more runners.
2. Every source-built service resolves to a Git commit and an immutable registry digest.
3. Prebuilt images must already use `repository@sha256:digest`; mutable tags remain forbidden.
4. Runtime secrets use `${secret:name}` and are resolved only when a release is dispatched.
5. Dependencies form an acyclic graph. URUFLOW computes the order; YAML map ordering is irrelevant.
6. Long-running services must become ready before dependants start.
7. Jobs must exit with status zero. Successful job containers are removed.
8. Created networks and volumes are persistent resources. A project delete never destroys data.
9. One project timeout covers Git, every build and push, and deployment; it is not reset per service.

## 2. Files

```text
/etc/uruflow/projects/urufi/
├── prod.yaml
└── prod.env
```

The directory and filename produce `urufi-prod`. The YAML file contains the complete build and runtime
model. Every built service declares its own source; prebuilt services declare an immutable image.

`prod.env` supplies non-secret interpolation and runtime values:

```ini
CONSOLE_HOST=console.urufi.net
LOG_LEVEL=info
VERSION=1.5.5
```

Supported interpolation forms are `${NAME}`, `${NAME:-default}`, `${NAME:?message}`, and `$$` for a
literal dollar sign. Secret references are preserved during interpolation.

## 3. Complete Environment Shape

```yaml
builder: controller-01
runners: [controller-01]
timeout: 2h

resources:
  networks:
    edge:
      name: traefik_net
      external: true
    data:
      name: urufi-data
      driver: bridge
      internal: true
      attachable: true
      labels:
        owner: urufi
  volumes:
    postgres_data:
      name: urufi-postgres-data
      driver: local

services:
  postgres:
    image: postgres@sha256:<64-lowercase-hex-digest>
    networks:
      data:
        aliases: [postgres]
    mounts:
      - type: volume
        source: postgres_data
        target: /var/lib/postgresql/data
    env:
      POSTGRES_DB: urufi_core
      POSTGRES_USER: urufi
      POSTGRES_PASSWORD: "${secret:urufi_db_password}"
    healthcheck:
      type: command
      command: [pg_isready, -U, urufi, -d, urufi_core]
      interval: 10s
      timeout: 5s
      retries: 10
      start_period: 10s
    resources:
      memory: 1GiB
      cpus: 2
      pids: 256
    security:
      no_new_privileges: true
    logging:
      driver: json-file
      options:
        max-size: 10m
        max-file: "3"

  migrate:
    git: git@gitlab.com:urufi/core.git
    branch: main
    dockerfile: Dockerfile
    context: .
    mode: job
    timeout: 5m
    command: [./urufi-core, migrate]
    networks:
      data: {}
    depends_on:
      postgres: healthy
    env:
      DB_HOST: postgres
      DB_PASSWORD: "${secret:urufi_db_password}"

  core:
    git: git@gitlab.com:urufi/core.git
    branch: main
    dockerfile: Dockerfile
    context: .
    entrypoint: [./urufi-core]
    command: [serve]
    networks:
      data:
        aliases: [urufi-core]
      edge: {}
    depends_on:
      migrate: completed
    env:
      APP_VERSION: "${VERSION:?VERSION is required}"
      DB_HOST: postgres
      DB_PASSWORD: "${secret:urufi_db_password}"
    healthcheck:
      type: http
      path: /api/v1/health
      port: 8080
      interval: 15s
      timeout: 5s
      retries: 5
      start_period: 20s
    labels:
      traefik.enable: "true"
      traefik.docker.network: traefik_net
      traefik.http.routers.core.rule: "Host(`${CONSOLE_HOST:?CONSOLE_HOST is required}`)"
      traefik.http.services.core.loadbalancer.server.port: "8080"
    resources:
      memory: 512MiB
    security:
      no_new_privileges: true
      read_only_rootfs: true
      cap_drop: [ALL]
```

The placeholder image digest in this example must be replaced by the reviewed digest. A mutable
`postgres:17-alpine` tag is rejected.

## 4. Build Sources

Every built service declares its repository and branch:

```yaml
services:
  api:
    git: git@gitlab.com:acme/api.git
    branch: release
    dockerfile: Dockerfile
    context: .
```

The builder keeps one checkout per Git URL and branch, builds each target inside its own source root,
and rejects Dockerfile or context paths that escape that root. Services sharing a source reuse the
same checkout. A release records the resolved commit for every built service alongside its image
digest. Service-owned projects are deployed manually.

## 5. Runtime Fields

The top-level `timeout` defaults to `2h` and caps the complete release. The service-level `timeout`
below applies only to a `job` and cannot extend the project deadline.

| Field | Meaning |
| :--- | :--- |
| `image` | Immutable prebuilt image; mutually exclusive with `dockerfile` |
| `git`, `branch` | Source repository and branch for a built service |
| `dockerfile`, `context`, `build_args` | Docker build input inside that source root |
| `entrypoint` | Exact OCI entrypoint string list |
| `command` | String runs through `sh -c`; string list is exact OCI `Cmd` |
| `mode` | `service` (default) or `job` |
| `timeout` | Maximum job duration |
| `depends_on` | Service name to `started`, `healthy`, or `completed` |
| `ports` | `host:container`, `host-ip:host:container`, optional `/tcp` or `/udp` |
| `volumes` | Backward-compatible `source:target[:ro]` binds |
| `mounts` | Typed `bind`, `volume`, or `tmpfs` mounts |
| `networks` | Logical resource names with optional DNS aliases |
| `env` | Service environment merged over project variables |
| `restart` | Docker restart policy; jobs always use `no` |
| `resources` | Memory bytes/units, fractional CPUs, and PID limit |
| `security` | User, read-only root, no-new-privileges, capability add/drop |
| `logging` | Docker log driver and options |
| `healthcheck` | Native HTTP, TCP, running, or Docker command readiness |
| `labels` | Docker labels; `uruflow.*` remains reserved |

Typed bind mounts do not create a missing host path unless explicitly requested:

```yaml
mounts:
  - type: bind
    source: /etc/urufi/core.yaml
    target: /app/config.yaml
    read_only: true
    bind:
      create_host_path: false
```

## 6. Networks, Volumes and DNS

`resources.networks` and `resources.volumes` use logical keys. Services refer to the key; the runner
uses the configured Docker object name. Without an explicit `name`, URUFLOW creates
`<project>-<environment>-<logical-key>` so unrelated projects do not collide. `external: true`
requires the object to exist and fails the release if it does not. Non-external resources are
created idempotently before any container is changed.

Every named service is automatically added as a network alias on each attached network. Explicit
aliases are additive. This makes `postgres`, `minio`, or `urufi-core` resolvable without coupling DNS
to URUFLOW's physical container names.

Resources are not automatically removed. Removing a project must not destroy a database volume or a
network shared with another project.

## 7. Dependencies and Jobs

```yaml
depends_on:
  database: healthy
  migrate: completed
```

- `started` expresses a long-running dependency; URUFLOW's release safety still requires it to pass
  normal readiness before the next service starts.
- `healthy` documents that readiness is required. A native/Docker health check should be declared.
- `completed` is valid only for a `mode: job` dependency.

Cycles and unknown services are rejected before a release is created. Jobs run with restart policy
`no`, must exit zero before dependants start, and have their stdout/stderr copied into the release log
before the container is removed. Timed-out and failed job containers are also removed after their logs
are captured. Job side effects are not
rollbackable: if a later service fails, containers are restored but a completed database migration
is not undone. Migrations must therefore be backward-compatible.

## 8. Readiness and Rollback

`http`, `tcp`, and `command` checks support `interval`, `timeout`, `retries`, and `start_period`.
`running` uses `stable_for`. A command string becomes `CMD-SHELL`; a list becomes Docker `CMD`.

All images and declared resources are prepared before replacement begins. Long-running services are
then replaced in dependency order. A readiness failure restores already-replaced services in reverse
order. Successful jobs are not reversed.

## 9. Workspace Editing

Use `project create <project> <environment>` or `project edit <project-environment>` in the workspace.
Both use the same internal editor. `Ctrl+S` validates, atomically saves, and reloads the file; a failed
full reload restores the previous version. Files written by configuration management remain supported;
run `project reload` after changing them externally.

Unknown YAML fields, dependency cycles, invalid resource sizes, bad host IPs, source paths escaping a
checkout, mutable image tags, and missing required interpolation values fail validation before a
release starts.

## 10. Compose Migration

Translate Compose concepts rather than running a Compose file inside an agent:

| Compose | Native URUFLOW |
| :--- | :--- |
| service `build` | service `git`/`branch` + `dockerfile`/`context` |
| tagged `image` | resolve and pin `repository@sha256:digest` |
| top-level networks/volumes | `resources.networks` / `resources.volumes` |
| service networks and aliases | `services.<name>.networks` |
| `depends_on` conditions | `started`, `healthy`, `completed` |
| init container | `mode: job` |
| `deploy.resources.limits` | native `resources` |
| `security_opt: no-new-privileges` | `security.no_new_privileges: true` |
| log driver/options | native `logging` |
| Compose `.env` interpolation | project `<env>.env` + supported `${...}` forms |
| Compose secrets in plaintext | URUFLOW secret store references |

Do not commit production credentials during conversion. Create replacements in the Secrets view and
reference only their names.
