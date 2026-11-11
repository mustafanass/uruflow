# Configuration

URUFLOW keeps server settings, agent enrollment and project definitions in separate files. This page is the reference for process configuration, listener addresses and local commands. The [build and runtime model](native-build-model.md) owns project YAML fields.

## Files and paths

| File                                                       | Purpose                                          |
| ---------------------------------------------------------- | ------------------------------------------------ |
| `/etc/uruflow/config.yaml`                                 | Server, registry and operator listeners          |
| `<config-directory>/defaults.yaml`                         | Shared project variables                         |
| `<config-directory>/projects/<project>/<environment>.yaml` | Complete project definition                      |
| `<config-directory>/projects/<project>/<environment>.env`  | Project variable overlay                         |
| `/etc/uruflow/agent.yaml`                                  | Root agent enrollment and local process settings |

The server uses `--config` / `-c`, then `URUFLOW_CONFIG`, then `/etc/uruflow/config.yaml`. Project paths are relative to that configuration file's directory.

The agent uses `--config` / `-c`, then `URUFLOW_AGENT_CONFIG`, then its platform default. Non-root and Windows agent paths are under the user's home directory; root Unix agents use the paths shown below.

## Server settings

`uruflow init --advertise uruflow.internal` writes this file. It holds the listeners and paths the server needs before it can open its database; settings that change while the server runs live in **Settings**. The following shows the configuration shape.

```yaml
server:
  host: 0.0.0.0
  ufp_port: 9001
  advertise: uruflow.internal
  data_dir: /var/lib/uruflow

registry:
  host: uruflow.internal
  port: 5000
  namespace: uruflow
  username: uruflow
  image: registry:2
  socket: /var/run/docker.sock

web:
  host: 127.0.0.1
  port: 9002
  tls: false
```

### Server and registry

| Field                | Default                | Meaning                                                                                    |
| -------------------- | ---------------------- | ------------------------------------------------------------------------------------------ |
| `server.host`        | `0.0.0.0`              | UFP bind address and fallback webhook bind address                                         |
| `server.ufp_port`    | `9001`                 | Agent link port                                                                            |
| `server.advertise`   | Required               | Server name or address issued to agents                                                    |
| `server.data_dir`    | `/var/lib/uruflow`     | Database, logs, PKI and registry data                                                      |
| `registry.host`      | Set by `init`          | Registry address reachable by agents; defaults to the advertise name during initialization |
| `registry.port`      | `5000`                 | Published registry port                                                                    |
| `registry.namespace` | `uruflow`              | Image repository namespace                                                                 |
| `registry.username`  | `uruflow`              | Registry account                                                                           |
| `registry.image`     | `registry:2`           | Managed registry image                                                                     |
| `registry.socket`    | `/var/run/docker.sock` | Server Docker socket                                                                       |

Built-service repositories use `<registry>/<namespace>/<project>-<service>`. Set the registry and advertise names to addresses agents can resolve and reach.

### Operator listener

| Field                 | Default                 | Meaning                                                            |
| --------------------- | ----------------------- | ------------------------------------------------------------------ |
| `web.host`            | `127.0.0.1`             | Browser and operator API bind address                              |
| `web.port`            | `9002`                  | Operator port, separate from UFP, webhook and registry ports       |
| `web.tls`             | `false`                 | Enable HTTPS; required for a non-loopback bind                     |
| `web.cert`, `web.key` | Server certificate pair | Optional custom certificate and key; configure together            |
| `web.public_url`      | Empty                   | Expected browser origin, including scheme and any non-default port |

For direct remote access:

```yaml
web:
  host: 0.0.0.0
  port: 9002
  tls: true
  cert: /etc/letsencrypt/live/flow.example.com/fullchain.pem
  key: /etc/letsencrypt/live/flow.example.com/privkey.pem
  public_url: https://flow.example.com:9002
```

For an HTTPS reverse proxy on the same host, keep `web.host: 127.0.0.1` and `web.tls: false`, set `web.public_url: https://flow.example.com`, and forward that origin to `127.0.0.1:9002`. Preserve the external Host header and allow SSE responses to stream without buffering. Set proxy idle timeouts to permit long-lived streams.

A non-local public origin must use HTTPS. Direct HTTPS supports HTTP/2. Built assets and the operator API share one origin.

### Registry and agent reporting

**Settings → Registry** and **Settings → Agent reporting** hold:

| Setting         | Default                   | Meaning                                                                                        |
| --------------- | ------------------------- | ---------------------------------------------------------------------------------------------- |
| Image cleanup   | `10`                      | Distinct successful images kept per service; `0` turns automatic cleanup off                   |
| Registry login  | Generated                 | Registry password, encrypted in the database; rotation sends the new login to connected agents |
| Report interval | Each agent's file, or set | How often agents report; `2`–`300` seconds                                                     |
| Reconnect delay | Each agent's file, or set | How long agents wait before reconnecting; `1`–`60` seconds                                     |

On first start, the server imports `registry.password` and `registry.retain_releases` from this file; remove them afterwards. Agent reporting stays with each agent's `agent.yaml` until an administrator saves it once; from then on, connected agents on 3.0.6 or later apply the shared values immediately. Rotating the registry password waits until no release is running, restarts the registry for a few seconds and blocks new releases until it finishes.

### Webhooks

Administrators manage webhooks in **Settings → Webhooks**, not in this file:

| Setting      | Default            | Meaning                                                     |
| ------------ | ------------------ | ----------------------------------------------------------- |
| Receiving    | On                 | Accept signed pushes, or refuse every delivery              |
| Bind address | `server.host`      | Webhook listener address                                    |
| Port         | `9000`             | Webhook and health listener port, separate from other ports |
| Path         | `/webhook`         | Push receiver path                                          |
| HTTPS        | On                 | Plain HTTP requires a loopback bind behind an HTTPS proxy   |
| Certificate  | Server certificate | Optionally an uploaded certificate chain and private key    |
| Secret       | Generated          | Provider verification secret, encrypted in the database     |

Changes apply without restarting the server; a new address, port or HTTPS mode restarts only the webhook listener. A regenerated secret is shown once, and the previous secret is still accepted for 24 hours so repository webhooks can be updated. Every change is recorded in the audit log.

On first start, the server imports `webhook.*` and `server.http_port` from `config.yaml` into the database, so existing repository webhooks keep working. The file values are not read afterwards; remove them.

The listener exposes `POST <path>` and `GET /health`. Public Git providers need a certificate they trust, either directly or at an HTTPS reverse proxy. For a local proxy, bind the listener to `127.0.0.1`, turn HTTPS off and terminate HTTPS at the proxy.

GitHub uses a push event, `application/json`, a delivery ID and `X-Hub-Signature-256`. GitLab uses a push event, event UUID and `X-Gitlab-Token`. Repeated delivery IDs are retained for 30 days and acknowledged without another attempt. Branch references use `refs/heads/*`.

**Recent deliveries** lists signed pushes from the last 30 days with their result: `started`, `queued`, `ignored` when no enabled branch matches, or `failed`. The panel also shows the last rejected request, such as a wrong secret or deliveries turned off.

Each project's **Settings** tab enables deploy on push per repository and branch. A matching push releases exactly the pushed commit. A push that arrives while the project is busy is queued, the newest commit wins, and it starts when the project becomes idle; the queue does not survive a server restart. The response lists each matched project as `started`, `queued` or `failed`. GitHub `ping` events are acknowledged. [Deployments](deployments.md) describes how releases start.

## Agent settings

The enrollment command writes identity, key, roles and server address. This is the root Unix agent shape:

```yaml
agent_id: <enrolled-agent-id>
key: <enrolled-agent-key>
roles: [runner]
data_dir: /var/lib/uruflow-agent
pid_file: /var/run/uruflow-agent.pid
log_file: /var/log/uruflow-agent.log
server:
  host: uruflow.internal
  port: 9001
  ca_cert: /etc/uruflow/ca.crt
  reconnect_sec: 5
  metrics_sec: 10
docker:
  socket: /var/run/docker.sock
```

| Field                        | Meaning                                                                |
| ---------------------------- | ---------------------------------------------------------------------- |
| `agent_id`, `key`            | Server-issued identity and shared key                                  |
| `roles`                      | `builder`, `runner`, or both; must match enrollment                    |
| `data_dir`                   | Local state; builders keep checkouts in `sources/`                     |
| `pid_file`, `log_file`       | Local process tracking and log output                                  |
| `server.host`, `server.port` | Outbound UFP destination; port defaults to `9001`                      |
| `server.ca_cert`             | Server trust root                                                      |
| `server.reconnect_sec`       | Reconnect delay until the server sends one; defaults to `5` seconds    |
| `server.metrics_sec`         | Reporting cadence until the server sends one; defaults to `10` seconds |
| `docker.socket`              | Docker socket; default follows the platform                            |

Non-root agent configuration and CA paths are `~/.config/uruflow/agent.yaml` and `~/.config/uruflow/ca.crt`. Local state, PID and logs use `~/.local/share/uruflow-agent`. Registry trust installation still needs permission to write Docker's certificate directory.

## Local commands

### Server

| Command                                 | Effect                                                  |
| --------------------------------------- | ------------------------------------------------------- |
| `uruflow`                               | Print help                                              |
| `uruflow init --advertise HOST`         | Create server configuration                             |
| `uruflow serve`                         | Run the persistent server                               |
| `uruflow admin reset-password USERNAME` | Recover a local administrator and revoke their sessions |
| `uruflow version`                       | Print the product version                               |

`init` also accepts `--registry-host` and `--data-dir`. Recovery generates a password by default or accepts `--password-stdin`; see [Security](security.md#administrator-recovery).

### Agent

| Command                 | Effect                              |
| ----------------------- | ----------------------------------- |
| `uruflow-agent init`    | Write enrollment configuration      |
| `uruflow-agent run`     | Run in the foreground               |
| `uruflow-agent stop`    | Signal the configured agent process |
| `uruflow-agent status`  | Report the configured agent process |
| `uruflow-agent version` | Print the product version           |

`init` accepts required `--id`, `--key`, `--server`, plus `--roles`, `--ca` and `--docker-socket`. Use the command issued by **Agents → Enroll agent** for the exact identity and key.

Related: [Operations](operations.md), [Projects](projects.md), [Security](security.md).
