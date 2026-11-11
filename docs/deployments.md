# Deployments

A release is the server's record of a build, rollout or restore attempt. This page follows a release from the browser request to runner outcomes and current workload observations.

## Start a release

Choose **Deploy** on a project to release its saved definition. The server checks the project, active-release reservation, required secrets, image references, agent roles and connectivity before claiming work.

Only one release may own a project at a time. The database records the specification and runner targets for that attempt. Later configuration edits do not replace that recorded specification.

```mermaid
flowchart TD
    request[Deploy saved project] --> validate[Validate and reserve project]
    validate --> workflow{Workflow}
    workflow -->|build_only or build_deploy| build[Resolve sources and prepare images]
    workflow -->|deploy_only| deploy[Dispatch recorded image digests]
    build --> buildonly{Build only?}
    buildonly -->|Yes| artifacts[Record successful artifacts]
    buildonly -->|No| deploy
    deploy --> runners[Runners prepare and apply services]
    runners --> outcomes[Record each runner outcome]
    outcomes --> settle[Settle release and publish observations]
```

Project deadlines and workflow requirements are defined in [Build and runtime model](native-build-model.md#project-fields).

## Build artifacts

For each source-built service, the assigned builder fetches its repository and branch and resolves the commit. Shared repository and branch inputs reuse a checkout. It compares the source and build definition with the latest successful release and reuses that service's recorded image digest when nothing relevant changed. Before dispatch, the server checks the recorded manifest by digest using headers only, without downloading image layers.

The default source comparison covers the build context, Dockerfile and Docker ignore files. For a shared repository, `watch` lists the repository-relative files and folders that affect one service. Include shared libraries, schemas and dependency lockfiles. Dockerfile and ignore-file changes are always checked. Missing history, changed build settings and missing manifests cause a build. Registry authentication, connectivity and server errors are reported rather than treated as missing images.

```yaml
services:
  console:
    git: git@github.com:example/platform.git
    branch: main
    dockerfile: console/Dockerfile
    context: .
    watch: [console, shared, package-lock.json]
  core:
    git: git@github.com:example/platform.git
    branch: main
    dockerfile: core/Dockerfile
    context: .
    watch: [core, shared, go.mod, go.sum]
```

Changing `console` builds its image; changing `shared` builds both. Watch entries are literal paths, with directories including their descendants. When the context is `.` and no watch list is set, any tracked repository change is relevant.

New builds push the commit tag and `latest`, then read the repository digest. Reused images keep their original build commit and digest. Every release retains the complete service specification and artifact set for rollback. The server checks the reporting agent, expected repository, digest format and service coverage before recording artifacts. Prebuilt services already carry their digest references.

The release's main revision comes from the first rebuilt service. If every image is reused, it shows the first resolved source revision. Per-service commits always identify the original image builds.

Select **Rebuild all images** in the Deploy dialog to build every source service with fresh base images and without Docker's build cache. This also refreshes build inputs outside Git, such as dependencies fetched during a Dockerfile step. Selective builds require the updated builder agent; keeping unchanged containers requires the updated runner agent.

Build output streams through UFP into persisted release logs. `build_only` finishes after successful artifact publication. `build_deploy` continues to the recorded runners. `deploy_only` begins with runner dispatch.

## Readiness and replacement

Each runner:

1. Validates the request and prepares declared networks and volumes.
2. Keeps owned, healthy containers whose image and resolved runtime configuration match; pulls the images needed for replacements before any replacement begins.
3. Follows dependency order for services and jobs.
4. Stops and renames each service container that needs replacement aside.
5. Creates its replacement and waits for readiness.
6. Removes the previous containers after the runner's complete service sequence succeeds.

The old container stops before its replacement starts. On replacement failure, the runner removes the new container and attempts to restore the previous one. If a later component fails, it restores earlier replacements in reverse order and reports any restoration error.

Environment values, secrets, ports, readiness policies and runtime resource changes replace the affected service even when its image is reused. Unchanged containers remain running during that sequence. Each successful runner result records its applied container IDs, including kept containers; the browser uses those identities to show the live release without changing the containers' original creation labels.

A native `http`, `tcp`, `command` or `running` healthcheck is the chosen readiness policy. The [model reference](native-build-model.md#readiness) owns probe fields and defaults. Without a native policy, Docker image health is used when present; otherwise the service must stay running for five seconds. This fallback is bounded by two minutes and the release context. Exit, unhealthy state or a restart fails the wait.

Failed startup output is copied to the release log before the replacement is removed. Inspect that log even when the failed container no longer appears in inventory.

## Jobs and resources

Jobs run with restart policy `no`, must exit zero, and capture their output before container cleanup. A failed or timed-out job stops dependent work. Container restoration does not reverse completed database changes; migration jobs should remain compatible with the previous application.

Networks and volumes are prepared separately from containers and persist after replacement or project container removal. External resources must exist before a runner can use them.

## Read the result

UFP request acceptance means the agent accepted work. The release completes only after its stages and runner outcomes settle.

| Outcome     | Meaning                                                                 |
| ----------- | ----------------------------------------------------------------------- |
| `building`  | Source and artifact work is in progress                                 |
| `releasing` | Runtime work is in progress on targets                                  |
| `succeeded` | Required stages and all targets succeeded                               |
| `failed`    | A stage failed, a target failed or a required runner became unavailable |

Targets retain their own status and message. Runners work independently, so a failed release can include successful targets running the new version. Use those outcomes and current observations to choose the next action.

On server startup, unfinished releases are closed as failed with `interrupted by a server restart`. The server marks agent observations offline until connections return. Current container inventory is refreshed by agent reports.

## Restore a successful release

Choose **Rollback** on the project or **Restore release** on an eligible release. The server creates a new release with trigger `rollback`, reuses the selected successful release's recorded images and specification, and skips source builds. The current project supplies the total timeout for the new attempt.

The runner follows the same preparation, readiness and replacement sequence. Retain the registry artifacts referenced by releases you may restore.

## Stop workloads

**Stop** stops the project's managed containers on its configured runners. The project must be idle and the required runners connected. Persistent volumes and registry artifacts remain available.

## Triggering and observation

Releases start from **Deploy** in the browser or from a Git push. Deploy on push is enabled per repository and branch in the project's **Settings** tab. A matching push builds the services from that source at exactly the pushed commit, with trigger `webhook`; other services keep their usual source and reuse rules. Listener setup and responses are documented in [Configuration](configuration.md#webhook-listener).

Open a release to inspect progress, runner targets, artifacts, its saved specification and logs. Open the project to compare that result with current runtime observations. Closing a browser tab leaves the release running.

Implementation: `internal/pipeline`, `internal/link`, `internal/agent/builder` and `internal/agent/runner`.

Related: [Projects](projects.md), [Operations](operations.md), [Troubleshooting](troubleshooting.md).

Rollback choices use project history, including the latest successful artifact releases when they are older than the recent history window. Runtime health does not change which recorded release a container belongs to.
