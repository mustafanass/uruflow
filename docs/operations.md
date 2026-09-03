# Operations

Running URUFLOW day to day: services, logs, state, backup, recovery and the maintenance it does not
do for you.

## 1. Running the Server

`uruflow serve` is the persistent control-plane process. It starts the registry, agent link, webhook
listener and a root-only local control socket. The operations workspace is a detachable client:

```bash
sudo uruflow console
```

Bare `sudo uruflow` does the same thing. `Ctrl+D` closes the workspace; `Ctrl+C` detaches a live
stream. The server keeps running, agents stay connected, and active releases continue.
`uruflow --headless` remains a compatibility alias for `uruflow serve` so an old unit continues to
start during an upgrade.

The server runs as root. It reads `/etc/uruflow/config.yaml`, writes `<data_dir>`, and needs the
Docker socket to host the registry, so open its workspace with `sudo`. The systemd unit already runs
the persistent process as root.

Startup order matters and is fixed:

1. reconcile previous state — mark all agents offline, fail any release left `building` or `releasing`
2. start the registry container and wait for it to answer
3. load project files from `projects/`
4. start the UFP listener
5. start the HTTPS listener

Reconciliation runs first so a release can never be observed in an impossible state after a crash.
The server refuses to start if the registry does not answer within 60 seconds.

### systemd

Install the units shipped in `packaging/systemd/`:

```bash
sudo install -m 0644 packaging/systemd/uruflow.service /etc/systemd/system/
sudo install -m 0644 packaging/systemd/uruflow-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
```

The server unit is:

```ini
# /etc/systemd/system/uruflow.service
[Unit]
Description=URUFLOW control plane
Wants=network-online.target
After=network-online.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/uruflow serve
Restart=on-failure
RestartSec=5
TimeoutStopSec=30
StandardOutput=journal
StandardError=journal
SyslogIdentifier=uruflow

[Install]
WantedBy=multi-user.target
```

```ini
# /etc/systemd/system/uruflow-agent.service
[Unit]
Description=URUFLOW build and release agent
Wants=network-online.target
After=network-online.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/uruflow-agent run
Restart=on-failure
RestartSec=5
TimeoutStopSec=30
StandardOutput=journal
StandardError=journal
SyslogIdentifier=uruflow-agent

[Install]
WantedBy=multi-user.target
```

Enable the appropriate unit on each machine:

```bash
sudo systemctl enable --now uruflow        # server host
sudo systemctl enable --now uruflow-agent  # each agent host
```

There is no reason to stop `uruflow.service` to operate it. The workspace connects to
`<data_dir>/control.sock`; it does not start another server or open the database itself. Closing the
workspace leaves the service and all running operations intact.

## 2. Logs

| Log | Location |
| :--- | :--- |
| Server | `<data_dir>/uruflow.log` |
| Agent | `log_file` from `agent.yaml`, by default `/var/log/uruflow-agent.log` |
| Release output | Stored in the database, readable with `release logs` in the workspace |
| Container output | Streamed live on request; not stored |

The server and agent write every process log line to both the configured file and stdout. Under
systemd, stdout is captured by journald. Workspace process streams are delivered separately through
the local control socket.

Follow either service without stopping it:

```bash
sudo journalctl -fu uruflow
sudo journalctl -fu uruflow-agent
```

The files remain available for retention, support bundles and environments without journald:

```bash
tail -f /var/lib/uruflow/uruflow.log
tail -f /var/log/uruflow-agent.log
```

## 3. Agents

In the server workspace:

```text
agent list
agent add web-03 --roles runner
agent remove web-03
```

On the target machine:

```bash
sudo uruflow-agent status                           # on the target machine
sudo uruflow-agent stop
```

For a systemd-managed agent, use `systemctl status|stop|restart uruflow-agent`; the agent CLI status
and stop commands are intended for foreground/manual runs.

`uruflow` always reads `/etc/uruflow/config.yaml` unless `--config` or `URUFLOW_CONFIG` says
otherwise, and needs root to reach it and the data directory. The agent's default path depends on the
user: as root it uses `/etc/uruflow/agent.yaml`, otherwise `~/.config/uruflow/agent.yaml` — so a
`uruflow-agent status` run without `sudo` reports on a different configuration from the one the
service is using.

### Agent Permissions

A builder needs `git`, the `docker` CLI, and **root**. Two of its steps require it:

- installing the registry CA into `/etc/docker/certs.d/<registry>/ca.crt`
- running `docker login`

A non-root agent logs `could not install the registry CA: permission denied` and pushes will fail with
a certificate error. A runner needs only access to the Docker socket, because it pulls through the
Docker API using credentials supplied over the link.

### Disconnection Behaviour

While an agent is disconnected:

- its containers keep running under their restart policy
- log lines it produces are **dropped, not buffered** — a gap in a release log usually means the link
  dropped
- releases targeting it fail with `agent disconnected`
- it is reported offline in the interface

An agent reconnects on its own every `reconnect_sec` (default 5). There is nothing to restart on the
server side when one drops.

## 4. Environment Variables and Secrets

Use `project variables <project>` for one project-scoped list. Write `NAME=value` for plain settings
and `secret NAME=value` for encrypted settings. Press `Ctrl+S` to validate and apply the list.

Secret values are encrypted with `pki/secrets.key` and cannot be read back after storage—the editor
shows their references when reopened. Replace a reference with a new value to rotate it. Values are
visible while first entered in the editor, but are never passed as command arguments or written to
the project file.

Commands operate while the service remains online and do not affect agent connectivity.

For what the store protects and what it does not, see [Security](security.md#7-secrets).

## 5. Registry

The registry runs as a managed container named `uruflow-registry`. Inspect it like any other:

```bash
docker ps --filter name=uruflow-registry
docker logs uruflow-registry
```

Browse its contents from the interface with `5`, or directly:

```bash
curl --cacert /var/lib/uruflow/pki/ca.crt \
     -u uruflow:<password> \
     https://<registry-host>:5000/v2/_catalog
```

### Garbage Collection

**URUFLOW does not reclaim registry disk.** Deleting a tag from the interface removes the manifest, but
the blobs remain until you run Docker's garbage collector inside the container:

```bash
docker exec uruflow-registry \
  bin/registry garbage-collect /etc/docker/registry/config.yml
```

Every build stores a new immutable image manifest and blobs, so `<data_dir>/registry` grows with releases
until you do this. Schedule it.

## 6. State and What It Costs to Lose

| Path | Contents | If lost |
| :--- | :--- | :--- |
| `<data_dir>/pki/ca.key` | The trust root | Every agent and the registry must be reissued and the new CA redistributed to every machine |
| `<data_dir>/pki/secrets.key` | The secret encryption key | **Every stored secret becomes unrecoverable.** They must be set again from their original source. |
| `<data_dir>/uruflow.db` | Agents, loaded projects, releases | Every agent must be re-enrolled and runtime history is gone; project YAML can be reloaded |
| `<config_dir>/projects/` | File-backed projects | Recoverable from version control if you keep them there |
| `<config_dir>/config.yaml` | Server settings, registry password, webhook secret | Reissue the password and update every git host |
| `<data_dir>/registry/` | Image blobs | Rollback targets are gone; the next build repopulates only the current commit |

Running containers are unaffected by losing any of these. Recovery is about regaining control, not
restarting workloads.

### Backup and Restore

Back up `<data_dir>` and `<config_dir>` together. The database uses WAL journalling, so copy it with
SQLite rather than `cp` on a running server:

```bash
sqlite3 /var/lib/uruflow/uruflow.db ".backup '/backup/uruflow.db'"
tar czf /backup/uruflow-pki.tgz -C /var/lib/uruflow pki   # includes ca.key and secrets.key
tar czf /backup/uruflow-config.tgz -C /etc uruflow
```

To restore onto a new host: put `config.yaml`, `projects/`, `pki/` and `uruflow.db` back in place and
start the server. Because the CA and agent keys are preserved, existing agents reconnect without being
re-enrolled.

There is no database `export` command. Project files can live in version control, but agents
and release history exists only in the database—which is why the database is worth backing up
even if all your projects are files.

## 7. Release History and Retention

**Release logs are never pruned.** The `release_logs` table grows with every line of build output you
ever produce. On a busy server, prune it periodically:

```bash
sqlite3 /var/lib/uruflow/uruflow.db \
  "DELETE FROM release_logs WHERE release_id IN (
     SELECT id FROM releases WHERE started_at < date('now','-30 days'));
   VACUUM;"
```

Deleting a release row removes its targets and logs by foreign key cascade.

## 8. Alerts

Alerts are raised from agent metrics, evaluated on every push (default every 10 seconds).

| Alert | Warning | Critical |
| :--- | :--- | :--- |
| CPU | 80% | 90% |
| Memory | 90% | 95% |
| Disk | 85% | 95% |

A managed container in any state other than `running` raises a critical alert. An agent going offline
raises one too. Alerts resolve automatically when the condition clears; view them in `5 SYSTEM`.

## 9. Upgrading the Binaries

[Upgrading](upgrading.md) covers the procedure: the order to follow, what the automatic schema
migration does, and what a move from 1.x involves.

Two points that catch people out: a running process keeps the code it started with, so replacing a
binary changes nothing until you restart it; and upgrading during a release closes that release as
failed.

## 10. Routine Maintenance

| Task | Frequency | Why |
| :--- | :--- | :--- |
| Registry garbage collection | Monthly, or when disk grows | Nothing reclaims blobs |
| Release log pruning | Monthly | The table only grows |
| Database and PKI backup | Daily | The only copy of agent keys and the CA |
| Certificate check | Every few years | Leaves last five years, the CA ten |
| Builder work directory review | Occasionally | Checkouts under `<agent data_dir>/sources` persist |

For keys and views, see [Terminal Interface](interface.md). For diagnosing specific failures, see
[Troubleshooting](troubleshooting.md).
