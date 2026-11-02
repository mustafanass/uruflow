# Build and runtime model

Project YAML describes the build inputs and runtime state for every service. URUFLOW validates the complete model before creating a release, orders dependencies itself, and sends the resolved specification to agents.

## Project fields

| Field                | Meaning                                                              |
| -------------------- | -------------------------------------------------------------------- |
| `workflow`           | `build_deploy`, `build_only` or `deploy_only`; inferred when omitted |
| `timeout`            | Total release deadline, default `2h`                                 |
| `builder`            | Enrolled builder name for source-build workflows                     |
| `runners`            | Enrolled runner names for deployment workflows                       |
| `env`                | Shared project environment                                           |
| `services`           | Required map of named services                                       |
| `resources.networks` | Declared Docker networks                                             |
| `resources.volumes`  | Declared Docker volumes                                              |

`build_only` needs a builder and source-built services, with no runners. `deploy_only` needs runners and pinned images for every service, with no builder. `build_deploy` needs both placement roles and at least one source-built service. Inference chooses build-and-deploy for built services with runners, build-only for built services without runners, and deploy-only for prebuilt services.

The total deadline covers source fetch, builds, pushes and runner deployment. A job timeout is also capped by the time remaining in that release.

Keep runtime settings on the service that owns them so each component's configuration is easy to review.

## Service fields

| Field           | Meaning                                                                      |
| --------------- | ---------------------------------------------------------------------------- |
| `git`, `branch` | Repository and branch owned by a source-built service                        |
| `dockerfile`    | Dockerfile inside that source root                                           |
| `context`       | Build context inside the source root; defaults to `.`                        |
| `build_args`    | String map of Docker build arguments                                         |
| `image`         | Prebuilt `repository@sha256:` reference with 64 lowercase hexadecimal digits |
| `entrypoint`    | Exact OCI entrypoint list                                                    |
| `command`       | Shell string or exact OCI command list                                       |
| `mode`          | `service` by default, or one-shot `job`                                      |
| `timeout`       | Job runtime deadline; default `10m`                                          |
| `depends_on`    | Service names mapped to dependency conditions                                |
| `ports`         | Published ports                                                              |
| `volumes`       | Shorthand mounts using `source:target[:ro]`                                  |
| `mounts`        | Typed bind, volume or tmpfs mounts                                           |
| `env`           | String map merged over project variables                                     |
| `network`       | Docker network selection                                                     |
| `networks`      | Declared network attachments and optional aliases                            |
| `restart`       | Docker restart policy; default `unless-stopped`, jobs use `no`               |
| `resources`     | Memory, CPU and PID limits                                                   |
| `security`      | User, capabilities and container security options                            |
| `logging`       | Docker log driver and string options                                         |
| `healthcheck`   | Native readiness policy                                                      |
| `labels`        | String Docker labels; the `uruflow.` prefix belongs to URUFLOW               |

Choose `dockerfile` with `git` and `branch`, or `image`. A built service's Dockerfile and context must remain inside its checkout. Services sharing a repository and branch reuse the builder checkout. Releases record source commits and artifact digests per service.

Published ports accept `host:container` or `host-ip:host:container`, followed by optional `/tcp` or `/udp`.

## Example: database, migration and application

This example uses `shop-production`. Supply the reviewed database digest through `production.env`, set `shop-production.DB_PASSWORD` from the variables editor in **Configuration**, and replace the repository and agent names with your own.

```yaml
workflow: build_deploy
builder: build-01
runners: [runner-01]
timeout: 2h
resources:
  networks:
    data:
      driver: bridge
  volumes:
    database:
      driver: local
services:
  database:
    image: "${DATABASE_IMAGE:?Set DATABASE_IMAGE to a full image digest}"
    networks:
      data:
        aliases: [database]
    mounts:
      - type: volume
        source: database
        target: /var/lib/postgresql/data
    env:
      POSTGRES_DB: shop
      POSTGRES_USER: shop
      POSTGRES_PASSWORD: "${secret:shop-production.DB_PASSWORD}"
    healthcheck:
      type: command
      command: [pg_isready, -U, shop, -d, shop]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 10s
  migrate:
    git: git@github.com:your-org/shop.git
    branch: main
    dockerfile: Dockerfile
    mode: job
    timeout: 5m
    command: [./shop, migrate]
    networks:
      data: {}
    depends_on:
      database: healthy
    env:
      DATABASE_PASSWORD: "${secret:shop-production.DB_PASSWORD}"
  api:
    git: git@github.com:your-org/shop.git
    branch: main
    dockerfile: Dockerfile
    command: [./shop, serve]
    ports: ["8080:8080"]
    networks:
      data: {}
    depends_on:
      migrate: completed
    env:
      DATABASE_PASSWORD: "${secret:shop-production.DB_PASSWORD}"
    healthcheck:
      type: http
      path: /health
      port: 8080
    resources:
      memory: 512MiB
      cpus: 1.5
      pids: 256
    security:
      no_new_privileges: true
      cap_drop: [ALL]
    logging:
      driver: json-file
      options:
        max-size: 10m
        max-file: "3"
```

Resolve the image digest using the Docker host that will pull it, then copy the complete reference into the variable file. This command can inspect an image you have pulled:

```sh
docker pull postgres:17
docker image inspect --format '{{index .RepoDigests 0}}' postgres:17
```

## Networks and volumes

Resource maps use logical keys. A missing resource `name` resolves to `<project>-<environment>-<key>`. Set `name` to use a specific Docker object. `external: true` requires that object to exist already.

Network definitions accept `name`, `driver`, `external`, `internal`, `attachable`, `options` and `labels`. Volume definitions accept `name`, `driver`, `external`, `options` and `labels`.

Services refer to logical network keys and receive their service name as a DNS alias on attached networks. Additional aliases are additive. Networks and volumes persist across container replacement and project container removal.

Typed mounts accept `type`, `source`, `target`, `read_only` and `bind.create_host_path`. For example:

```yaml
mounts:
  - type: bind
    source: /etc/shop/config.yaml
    target: /app/config.yaml
    read_only: true
    bind:
      create_host_path: false
  - type: tmpfs
    target: /tmp
```

## Dependencies and jobs

| Condition   | Requirement                                                         |
| ----------- | ------------------------------------------------------------------- |
| `started`   | Long-running dependency starts and passes normal rollout readiness  |
| `healthy`   | Dependency passes readiness; configure its health policy explicitly |
| `completed` | Dependency is a job that exits successfully                         |

The dependency graph must be acyclic and refer to declared services. YAML map order does not decide rollout order.

Jobs run with restart policy `no`, finish with exit code zero, and capture output into the release log before their containers are removed. Timeout or a nonzero exit fails the release. Design migration jobs so the previous application can still use the resulting database state if container restoration is needed.

## Readiness

| Type      | Required fields       | Optional fields                                            |
| --------- | --------------------- | ---------------------------------------------------------- |
| `http`    | `path`, `port`        | `scheme`, `interval`, `timeout`, `retries`, `start_period` |
| `tcp`     | `port`                | `interval`, `timeout`, `retries`, `start_period`           |
| `command` | `command`             | `interval`, `timeout`, `retries`, `start_period`           |
| `running` | Positive `stable_for` | —                                                          |

HTTP accepts `2xx`. The default HTTP scheme is `http`; probe defaults are `5s` interval, `3s` timeout and 10 attempts. A command string becomes Docker `CMD-SHELL`; a list becomes `CMD`.

The selected native healthcheck owns rollout readiness. Without one, Docker image health or the fallback running window is used. [Deployments](deployments.md#readiness-and-replacement) explains the runtime sequence.

## Resource and security controls

`resources.memory` accepts bytes or supported units such as `512MiB`; `cpus` accepts fractional CPU counts, and `pids` limits process count.

`security` accepts `user`, `read_only_rootfs`, `no_new_privileges`, `cap_add` and `cap_drop`. `logging.driver` and `logging.options` are passed to Docker. Generic labels support reverse proxies and monitoring tools; URUFLOW's own ownership labels are reserved.

## Implementation

The file schema lives in `internal/projects/schema.go`. Parsing and validation live beside it. Resolved types live in `internal/models`; UFP runtime payloads live in `internal/ufp/payload.go`. Builders and runners apply them under `internal/agent`.

Related: [Projects](projects.md), [Deployments](deployments.md), [Security](security.md).
