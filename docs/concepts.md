# Concepts

These terms describe the same objects in the browser, project files and server code.

## Control and execution

The **server** owns desired configuration, access, release history and fleet coordination. It serves the browser workspace, receives agent connections, and manages the private registry.

An **agent** connects outward to the server over TLS UFP. Its enrolled roles determine the work it may accept:

| Role    | Work                                                                 | Host requirements                                |
| ------- | -------------------------------------------------------------------- | ------------------------------------------------ |
| Builder | Fetch source, build images, publish artifacts                        | Docker Engine, Docker CLI, Git and source access |
| Runner  | Pull image digests, prepare resources, execute jobs and run services | Docker Engine and socket access                  |

One agent can hold both roles. Agents apply work locally and report observations to the server.

## Project and environment

A **project** is a release unit containing services and their build or runtime definitions. An **environment** is the filename that identifies one independently deployed instance.

```text
projects/shop/production.yaml → shop-production
projects/shop/staging.yaml    → shop-staging
```

Each expanded project has its own placement, configuration and release history. The server resolves enrolled agent names from files into agent identities.

## Service and job

A **service** describes one application component. It either builds from its own Git source and Dockerfile or uses a prebuilt image digest. A long-running service becomes a container on each runner.

A **job** is a service with `mode: job`. It runs to completion, such as a database migration. Dependencies select when another component may start. The [build and runtime model](native-build-model.md) defines those conditions.

## Workflow and release

A **workflow** selects the stages a project needs:

| Workflow       | Result                                        |
| -------------- | --------------------------------------------- |
| `build_only`   | Build and publish artifacts                   |
| `deploy_only`  | Run prebuilt image digests                    |
| `build_deploy` | Build artifacts, then release them to runners |

A **release** records one attempt: its saved specification, commits, image digests, timestamps, status and runner outcomes. A **rollback** creates a new release from a successful release's recorded artifacts and specification.

A saved file describes intent. A succeeded release describes the result of that attempt. Current container observations describe what is running now.

## Image, container and resource

An **image** is a build artifact or prebuilt artifact addressed as `repository@sha256:digest`. Tags such as `latest` are convenient registry pointers; the pipeline records digests for deployment.

A **container** is a runtime instance on a runner. URUFLOW uses ownership labels and full Docker identities to associate it with a project, service and release.

A **resource** is a declared Docker network or volume. Resource definitions and attachments form part of a project's runtime model. Their data can outlive container replacement.

The **registry** is the TLS-protected image store managed by the server. Builders publish there, and runners receive the address, credentials and trust root through UFP.

## People and observations

A **role** describes a user's permitted actions. A **project grant** describes which projects they can access. User roles and agent roles are separate controls; see [Security](security.md).

An **observation** is a reported sample with collection time and availability. Infrastructure graphs combine observations with project definitions and release records. They expose expected placement, current instances and relationships without a separate graph configuration.

Related: [Architecture](architecture.md), [Projects](projects.md), [Browser workspace](interface.md).
