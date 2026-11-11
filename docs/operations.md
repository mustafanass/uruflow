# Operations

Run the server and agents as persistent services. The browser starts server-owned work and reads observations; its lifetime does not control releases or Docker workloads.

## Services

Install the built binaries as `/usr/local/bin/uruflow` and `/usr/local/bin/uruflow-agent` on their respective machines. On Linux, a server unit can use:

```ini
[Unit]
Description=URUFLOW server
Wants=network-online.target
After=network-online.target docker.service
Requires=docker.service

[Service]
ExecStart=/usr/local/bin/uruflow serve --config /etc/uruflow/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

The agent unit uses the same dependency and restart settings with:

```ini
[Service]
ExecStart=/usr/local/bin/uruflow-agent run --config /etc/uruflow/agent.yaml
Restart=on-failure
RestartSec=5
```

Install units under `/etc/systemd/system/`, then reload and enable the relevant service:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now uruflow
```

Use `uruflow-agent` as the service name for the agent unit. Follow logs with `journalctl -fu uruflow` or `journalctl -fu uruflow-agent`.

### Agent permissions

The agent needs its Docker socket, local state and log paths, and permission to install registry trust under `/etc/docker/certs.d/<registry>/ca.crt`. Root Unix service operation supplies these permissions. Builders additionally run Git and the Docker CLI under the service account, so configure source credentials for that account.

## Routine work

| Task                                 | Workspace location                      |
| ------------------------------------ | --------------------------------------- |
| Deploy, stop or restore a project    | Project header and release history      |
| Update YAML or variables             | Project → Configuration                 |
| Apply external file edits            | Projects → Reload definitions           |
| Inspect host and workload resources  | Agents → Containers and System → Server |
| Follow rollout output                | Release Logs tab                        |
| Inspect a running or exited workload | Container detail tabs                   |
| Enroll or replace an agent           | Agents                                  |
| Manage accounts and grants           | System → Access                         |
| Inspect changes and conditions       | Activity timeline and its Alerts view   |

[Browser workspace](interface.md) describes the controls. [Security](security.md) owns account and recovery procedures.

## State and logs

The server data directory contains:

```text
/var/lib/uruflow/
  uruflow.db
  uruflow.log
  web-setup-token          present before first administrator setup
  pki/
    ca.crt
    ca.key
    server.crt
    server.key
    registry.crt
    registry.key
    htpasswd
    secrets.key
  registry/
```

The database stores accounts, sessions, grants, audit, enrollment, loaded projects, releases, targets, release logs and observations. Project files remain beside `config.yaml` in its `projects/` directory.

Agent logs use `log_file` from `agent.yaml`; builders keep checkouts under `<agent-data-dir>/sources/`. Release logs persist on the server. Container logs come from Docker through UFP; download the browser's loaded buffer when keeping it for diagnosis.

## Registry maintenance

URUFLOW manages the registry container and stores blobs under `<data_dir>/registry`. **System → Server** reads repository names and tags from a persistent server cache. It refreshes after releases and every three minutes. Digest and size load when an image is opened; incomplete metadata shows **Size unknown**. Docker manifests, OCI manifests and image indexes are supported.

Automatic cleanup keeps the latest ten distinct successful images per service by default. Running or last-observed container images, saved pinned images, `latest`, manual tags and shared image-index children remain protected. Change the window in **Settings → Registry**, or set `0` to turn automatic cleanup off. A new window applies from the next cleanup run. Older release records and logs remain; a release whose artifacts expired no longer offers restoration.

Cleanup first checks ten minutes after startup, then daily. Busy installations retry after releases and project edits finish. The pipeline excludes new releases and configuration writes during cleanup. URUFLOW removes obsolete manifests, stops registry writes by stopping the registry container, collects unreferenced blobs in an isolated container, and restarts the registry. Registry health shows **maintenance** during collection. A failed or interrupted collection is stopped before writes resume.

Tags can share a digest. Manual manifest deletion can remove several tags and still needs a review of release references. Automatic cleanup only considers recorded URUFLOW artifacts in its configured registry namespace. Incomplete registry or history reads prevent automatic deletion.

## Backup and restore

Keep one coordinated backup set containing:

| Material                          | Why it belongs                                                  |
| --------------------------------- | --------------------------------------------------------------- |
| `config.yaml` and `defaults.yaml` | Listener settings, credentials and shared variables             |
| Entire `projects/` directory      | Authoritative YAML and variable overlays                        |
| `uruflow.db`                      | Enrollment, access, release history, audit and encrypted values |
| Entire `pki/` directory           | CA identity, registry trust and secret encryption key           |
| Entire `registry/` directory      | Recorded deployment artifacts                                   |

Pause changes and wait for active releases to finish. For a filesystem copy, stop the server and quiesce the managed registry before copying their storage. Protect the backup as sensitive installation state.

Restore the matching files, database, PKI and registry data to a stopped installation. Preserve private file permissions, then start the server and agents. Verify sign-in, CA trust, recent agent samples, project definitions, registry access and release history before deploying. Test restoration in an isolated environment as part of operational validation.

## Restarts and upgrades

Server startup reconciles interrupted releases and waits for agents to report again. Valid browser sessions persist, and live views reconnect. Docker workloads continue on their hosts during control-plane service replacement.

Use the coordinated [URUFLOW 3 upgrade procedure](upgrading.md). Keep server, agents and the embedded frontend from the same release.

Related: [Configuration](configuration.md), [Deployments](deployments.md), [Troubleshooting](troubleshooting.md).
