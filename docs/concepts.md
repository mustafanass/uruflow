# Core Concepts

This page defines the vocabulary used throughout the URUFLOW documentation. Read it before the
configuration reference — most confusion about URUFLOW comes from the difference between a *project*
and an *environment*, or between the *control plane* and the *execution plane*.

## The Two Planes

URUFLOW separates the machine that decides what should happen from the machines that do it.

| Plane | Component | Holds |
| :--- | :--- | :--- |
| Control plane | URUFLOW Server | Project definitions, release history, agent identities, the registry, the certificate authority |
| Execution plane | Agents | Source checkouts (builders only), images, running containers |

The control plane never runs your workloads. The execution plane never decides what to run.

This split has a consequence worth stating early: **if the server stops, running containers keep
running.** Agents reconnect when it returns. What you lose while it is down is the ability to start
new releases, not the workloads themselves.

## Server

A single process that owns:

- the SQLite database — agents, projects, releases, logs, containers, alerts
- the certificate authority and all issued certificates
- the private registry container
- the UFP listener that agents connect to
- the HTTPS listener that receives webhooks
- the root-only local command transport used by the single-page operations workspace

There is exactly one server. It is a single point of failure for *starting* work, not for *running* it.

## Agent

A process running on a target machine, connecting outward to the server over UFP. Agents are never
dialled by the server, so they can sit behind NAT.

An agent declares its roles when it connects. They must exactly match its enrollment, and both peers
refuse operations outside those roles.

### Builder

A builder may see source code. It clones the repository, runs `docker build`, tags the image, and
pushes it to the registry. A builder needs `git` and the `docker` CLI.

### Runner

A runner never sees source code, a Dockerfile, or a build toolchain. It pulls one digest-pinned image and
runs it. A runner needs only a Docker socket.

One agent may hold both roles. One builder can serve every project.

## Project

A project is one repository deployed to one set of runners. It owns:

- a git source and branch
- how to build it — Dockerfile path, build context, build arguments
- which agent builds it, and which agents run it
- the runtime shape — ports, volumes, network, environment variables, restart policy
- whether webhook pushes deploy it automatically

A project produces one image and one container per runner. A project that declares a `services` block
produces one of each per service. See [Projects](projects.md).

## Environment

An environment is a name such as `dev`, `stg` or `prod`. **URUFLOW has no environment type.** A file
called `projects/api/dev.yaml` produces an ordinary project named `api-dev`.

This is a deliberate design decision. Environments exist in the file format, where they remove
duplication, and are expanded into flat projects before anything else sees them. The pipeline, the
runner, the registry and the release history never learn the word.

The practical effects:

- `api-dev` and `api-prod` are separate projects with separate release histories
- they run as separate containers, `uruflow-api-dev` and `uruflow-api-prod`
- they push to separate image repositories, so `api-dev:latest` and `api-prod:latest` are unrelated
- deploying one does not touch the other

See [Projects](projects.md) for the file format.

## Release

A release is one attempt to build a commit and roll the resulting image out. It records the image
reference, the resolved commit, the trigger, a status, and a per-runner outcome.

A release moves through two stages:

1. **Build** — one builder clones, builds, tags and pushes
2. **Release** — every runner pulls that exact image and runs it

A release that fails during the build never reaches any runner.

See [Deployments](deployments.md) for the full lifecycle.

## Image

The registry catalog exposes two convenient tags for each build:

```text
<registry>/<namespace>/<project>:<commit>     12-character commit SHA, immutable
<registry>/<namespace>/<project>:latest       moving pointer to the most recent build
```

The deployment unit is the `repository@sha256:digest` reference, not either tag. Releases record the
immutable digest. Rollback re-releases a recorded digest rather than rebuilding a
commit, so what returns is byte-identical to what ran before.

## Registry

A `registry:2` container that URUFLOW starts and manages itself, with TLS and password
authentication. You do not supply one. Agents receive its address, credentials and CA certificate
over the authenticated agent link.

See [Operations](operations.md#5-registry) for its storage and garbage collection behaviour.

## Container

URUFLOW only manages containers carrying its ownership labels:

```text
uruflow.managed = "true"
uruflow.project = "api-prod"
uruflow.release = "9f3c2a1b"
uruflow.role    = "service"    (or "registry")
```

Agents report only labelled containers, and only labelled containers are ever stopped or removed.
Anything else on the machine is invisible to URUFLOW.

## UFP

The binary protocol agents and the server speak over TLS. It carries build and release instructions,
streaming logs, and metrics. See [UFP Protocol](protocol.md).

## Terminology

These terms are used consistently and mean one thing each.

| Term | Meaning |
| :--- | :--- |
| URUFLOW Server | The control-plane process |
| Agent | A process on a target machine |
| Builder | An agent role permitted to build |
| Runner | An agent role permitted to run |
| Project | One repository deployed to one set of runners |
| Environment | A name that expands into a separate project |
| Release | One build-and-rollout attempt |
| Image | A digest-addressed artifact in the registry |
| Container | A running instance of an image |
| Registry | The private `registry:2` instance URUFLOW manages |
| UFP | The server/agent wire protocol |

"Deployment" is used informally for the act of releasing. The recorded object is always a *release*.
