# Operations

Running URUFLOW day to day: services, logs, state, backup, recovery and the maintenance it does not
do for you.

## 1. Running the Server

`uruflow` starts the registry, the agent link, the webhook listener and the terminal interface in one
process. `--headless` starts everything except the interface.

The server runs as root. It reads `/etc/uruflow/config.yaml`, writes `<data_dir>`, and needs the
Docker socket to host the registry, so every `uruflow` command below is run with `sudo` — or from a
unit, which already runs as root.

Startup order matters and is fixed:

1. reconcile previous state — mark all agents offline, fail any release left `building` or `releasing`
2. start the registry container and wait for it to answer
3. load project files from `projects/`
4. start the UFP listener
5. start the HTTP listener

Reconciliation runs first so a release can never be observed in an impossible state after a crash.
The server refuses to start if the registry does not answer within 60 seconds.

### systemd

```ini
# /etc/systemd/system/uruflow.service
[Unit]
Description=uruflow server
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/uruflow --headless
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```ini
# /etc/systemd/system/uruflow-agent.service
[Unit]
Description=uruflow agent
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/uruflow-agent run
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Run `uruflow` on a terminal when you want the interface. Both read the same configuration, but only
one process may own the listeners at a time — stop the unit first.

## 2. Logs

| Log | Location |
| :--- | :--- |
| Server | `<data_dir>/uruflow.log` |
| Agent | `log_file` from `agent.yaml`, by default `/var/log/uruflow-agent.log` |
| Release output | Stored in the database, readable in the releases view |
| Container output | Streamed live on request; not stored |

The server never writes to stdout while the interface is running — logs would corrupt the display.
Under `--headless` it writes to the log file, falling back to stdout only if that file cannot be
opened.

Follow the server log while using the interface:

```bash
tail -f /var/lib/uruflow/uruflow.log
```

## 3. Agents

```bash
sudo uruflow agent list                             # on the server
sudo uruflow agent add web-03 --roles runner
sudo uruflow agent remove web-03

sudo uruflow-agent status                           # on the target machine
sudo uruflow-agent stop
```

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

## 4. Secrets

Secrets are managed only from the interface. Press `7`, then `n` to store one and `d` to remove one.

Values are encrypted with `pki/secrets.key` and cannot be read back — the interface shows names and
masks only. To change a secret, store it again under the same name.

Because the interface and `--headless` cannot both hold the listeners, changing a secret on a server
running as a service means stopping the unit, running `uruflow` on a terminal, and starting the unit
again. Plan secret changes as maintenance rather than as a live operation.

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

Every build pushes a new immutable tag, so `<data_dir>/registry` grows with every release forever
until you do this. Schedule it.

## 6. State and What It Costs to Lose

| Path | Contents | If lost |
| :--- | :--- | :--- |
| `<data_dir>/pki/ca.key` | The trust root | Every agent and the registry must be reissued and the new CA redistributed to every machine |
| `<data_dir>/pki/secrets.key` | The secret encryption key | **Every stored secret becomes unrecoverable.** They must be set again from their original source. |
| `<data_dir>/uruflow.db` | Agents, projects, releases | Every agent must be re-enrolled by hand; standalone projects and all history are gone |
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

There is no `export` or `apply` command. File-backed projects can live in version control, but agents
and standalone projects exist only in the database — which is why the database is worth backing up
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
raises one too. Alerts resolve automatically when the condition clears; view them with `6`.

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
