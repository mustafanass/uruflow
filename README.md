<h1 align="center">URUFLOW</h1>

<p align="center">
  A self-hosted deployment control plane for Docker workloads.
</p>

<p align="center">
  <a href="https://github.com/mustafanass/uruflow/releases"><img src="https://img.shields.io/badge/release-v2.2.0-2DD4BF?style=flat-square" alt="release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-mit-2DD4BF?style=flat-square" alt="license"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/go-1.25.13-00ADD8?style=flat-square" alt="go"></a>
  <a href="docs/protocol.md"><img src="https://img.shields.io/badge/protocol-ufp%2F3-F5A524?style=flat-square" alt="ufp protocol"></a>
</p>

<br>

URUFLOW builds a commit once on a builder machine, stores the resulting image in a private registry it
runs itself, and releases that exact image across one or more runner machines. Builders and runners are
separate roles with different authority: builders see source code, runners receive only images.

It is operated from a terminal interface and communicates with agents over UFP, a small binary protocol
carried on TLS.

<p align="center">
  <a href="assets/uruflow-arch.png">
    <img src="assets/uruflow-arch.png" alt="URUFLOW architecture: the server, builder and runner agents and the private registry; the UFP frame and envelopes; the deployment flow; and the components on each side">
  </a>
</p>

<p align="center">
  <sub>Click to enlarge. The release lifecycle is detailed in
  <a href="docs/deployments.md">Deployments</a>, the wire format in
  <a href="docs/protocol.md">UFP Protocol</a>.</sub>
</p>

## Why URUFLOW

Most small deployment tools build on every target machine. Each server then needs your source code, a
build toolchain and spare capacity, and you cannot be certain two machines are running the same bytes.

URUFLOW builds once and ships the artifact:

| | Build on each target | URUFLOW |
| :--- | :--- | :--- |
| Source code | On every machine | On the builder only |
| Build toolchain | On every machine | On the builder only |
| Identical bytes everywhere | Not guaranteed | Guaranteed — one image, released by digest |
| Rollback | Rebuild an old commit | Re-release a stored image |
| Cost of adding a machine | Another full build | One pull |

## How It Works

A release runs in two stages.

**Build.** The server sends `build.run` to the project's builder. The builder clones the repository,
checks out the commit, runs `docker build`, tags the image with the 12-character commit SHA and with
`latest`, pushes both to the registry, and records the resulting immutable digest. Build output streams back as it happens.

**Release.** The server validates the registry digest and sends `release.run` to every runner. Each pulls that exact image and replaces
its container under the safety procedure below. A release completes when every runner has reported.

Rollback skips the build stage entirely and re-releases the image recorded on an earlier successful
release, so what returns is byte-identical to what ran before.

For the complete lifecycle and every failure boundary, see [Deployments](docs/deployments.md).

## Quick Start

The server needs Docker and root — it writes `/etc/uruflow` and `/var/lib/uruflow` and drives the
Docker socket to host the registry. A runner needs Docker; a builder needs Docker, `git` and root.

On the server:

```bash
curl -fsSL https://github.com/mustafanass/uruflow/releases/latest/download/uruflow-linux-amd64 -o uruflow
chmod +x uruflow && sudo mv uruflow /usr/local/bin/

sudo uruflow init     # writes /etc/uruflow/config.yaml
sudo uruflow          # starts the registry, the agent link, webhooks and the interface
```

To build from source instead, see [Development](#development).

Enrol an agent — press `3` then `n` in the interface, or:

```bash
sudo uruflow agent add builder-01 --roles builder,runner
```

URUFLOW prints the exact command to run on the target machine, then:

```bash
sudo uruflow-agent init --id <id> --key <key> --server uruflow.internal:9001 --roles builder,runner

scp <server>:/var/lib/uruflow/pki/ca.crt /tmp/ca.crt
sudo mv /tmp/ca.crt /etc/uruflow/ca.crt

sudo uruflow-agent run
```

Press `2` then `n` to add a project, `ctrl+s` to save, `enter` to deploy.

The full walkthrough, including rollback, is in [Getting Started](docs/getting-started.md).

## Core Concepts

| Concept | Meaning |
| :--- | :--- |
| **Server** | The control plane. Holds state, the certificate authority and the registry. Runs no workloads. |
| **Agent** | A process on a target machine. Connects outward to the server. |
| **Builder** | An agent role permitted to clone source and build images. |
| **Runner** | An agent role permitted to pull and run images. Never receives source. |
| **Project** | One repository deployed to one set of runners. Produces one image and one container per runner. |
| **Environment** | A name such as `dev` or `prod` that expands into a separate project. Not a type in the system. |
| **Release** | One attempt to build a commit and roll the image out, with a per-runner outcome. |
| **Registry** | A `registry:2` instance URUFLOW starts, secures and manages itself. |

The server decides; agents execute. If the server stops, running containers keep running — you lose
the ability to start new releases, not the workloads.

See [Core Concepts](docs/concepts.md).

## Release Safety

A runner never destroys what is running until the replacement has proven itself:

```text
pull the new image                     ← fails here, nothing has changed
stop the running container
rename it to <project>-previous        ← set aside, not deleted
start the replacement
check readiness
  ├─ ready      → remove the set-aside container
  └─ not ready  → remove the replacement, rename the previous back, start it
```

A service may define native HTTP, TCP or stable-running readiness. That policy is authoritative when
present. Without one, the existing Docker `HEALTHCHECK` or five-second running fallback is preserved.
Any readiness failure restores the previous container; multi-service releases restore every service
already replaced on that runner.

**This is not zero-downtime deployment.** A project owns a host port, so the old and new containers
cannot overlap — there is roughly a second of unavailability per release. What the procedure guarantees
is that a failed release leaves the previous version running rather than nothing at all.

See [Deployments](docs/deployments.md#3-release-safety).

## Example Project

Two environments of one service, defined as files:

```yaml
# projects/api/project.yaml — shared by every environment
git: git@github.com:acme/api.git
dockerfile: Dockerfile
context: .
```

```yaml
# projects/api/dev.yaml — becomes the project api-dev
branch: develop
builder: builder-01
runners: [dev-01]
auto_deploy: true
ports: ["8081:80"]
```

```yaml
# projects/api/prod.yaml — becomes the project api-prod
branch: main
builder: builder-01
runners: [web-01, web-02]
auto_deploy: false
ports: ["80:80"]
```

```ini
# projects/api/dev.env
LOG_LEVEL=debug
DATABASE_URL=postgres://dev-host/api
```

A push to `develop` deploys `api-dev` automatically. A push to `main` does nothing, because production
is released deliberately.

Projects can equally be created in the interface without writing files. See [Projects](docs/projects.md).

## Features

- Build once on a builder, release the identical image to every runner
- A private registry with TLS and authentication, managed by URUFLOW
- One certificate authority for both the agent link and the registry, distributed automatically
- Agent authentication by HMAC challenge-response; the shared key never crosses the wire
- Roles that constrain authority — a runner cannot be asked to build
- Health-gated releases that restore the previous container on failure
- Rollback by re-releasing a stored image, without rebuilding
- Environments as separate projects, defined by files or in the interface
- Environment variables merged from shared defaults down to a per-environment `.env`
- Encrypted secret storage, referenced as `${secret:name}` so project files stay safe to commit
- Multi-service projects — an app, a worker and a prebuilt dependency in one project
- Webhook deployment matched on git URL and branch, so one repository can feed several projects
- Live streaming of build output and container logs over a persistent connection
- One release per project at a time, enforced across restarts
- A terminal interface that works over SSH with no port forwarding

## Documentation

| Document | For |
| :--- | :--- |
| [Getting Started](docs/getting-started.md) | Installing and reaching a first deployment |
| [Core Concepts](docs/concepts.md) | The vocabulary and the two planes |
| [Terminal Interface](docs/interface.md) | Views, keys and status symbols |
| [Projects](docs/projects.md) | Defining projects, environments and variables |
| [Configuration](docs/configuration.md) | Every file, field, command and webhook setting |
| [Deployments](docs/deployments.md) | The release lifecycle, safety and failure boundaries |
| [Operations](docs/operations.md) | Services, logs, state, backup and maintenance |
| [Troubleshooting](docs/troubleshooting.md) | Symptoms, causes and fixes |
| [Security](docs/security.md) | Trust model, authentication and assumptions |
| [Architecture](docs/architecture.md) | System model, boundaries and invariants |
| [UFP Protocol](docs/protocol.md) | Framing, envelopes and the handshake |
| [Upgrading](docs/upgrading.md) | Within 2.x, and migrating from 1.x |

## Status

Server and agent binaries must use the same UFP wire format.

## Supported Platforms

| Platform | Server | Agent | Status |
| :--- | :---: | :---: | :--- |
| Linux (amd64) | ✓ | ✓ | Stable |
| Linux (arm64) | ✓ | ✓ | Beta |
| macOS (Apple silicon) | — | ✓ | Beta |
| macOS (Intel) | — | ✓ | Beta |

The server hosts the registry through a Docker socket and targets Linux. Agents run anywhere Docker
does; macOS agents are useful for development but are not a tested deployment target.

## Development

```bash
go build -o uruflow ./cmd/uruflow-server
go build -o uruflow-agent ./cmd/uruflow-agent

go test ./...                          # fast suite, no Docker required
URUFLOW_DOCKER_TESTS=1 go test ./...   # adds tests against a real Docker daemon
```

`cmd/uruflow-server` builds the `uruflow` command and `cmd/uruflow-agent` builds `uruflow-agent`.

| Package | Responsibility |
| :--- | :--- |
| `internal/ufp` | The wire contract: framing, envelopes, handshake, shared connection |
| `internal/link` | Server side of the agent link: sessions, event fan-out, metrics |
| `internal/pipeline` | Two-stage release orchestration |
| `internal/projects` | Project file loading, writing and dotenv parsing |
| `internal/registry` | Registry lifecycle and catalog |
| `internal/pki` | Certificate authority and leaf certificates |
| `internal/docker` | Docker Engine API client, shared by server and agent |
| `internal/storage` | Persistence contract and SQLite implementation |
| `internal/agent` | Agent daemon, builder, runner, metrics |
| `internal/tui` | Terminal interface |

`internal/ufp` depends on nothing else in the repository. It defines the contract both sides
implement, and adding an import there should be treated as a design change.

Where a change belongs is mapped in [Architecture](docs/architecture.md#12-where-a-change-belongs).

## Contributing

Issues and pull requests are welcome. [Contributing](CONTRIBUTING.md) covers the test suites, the
boundaries worth knowing before you start, and the conventions the codebase follows.

## License

MIT — see [LICENSE](LICENSE).
