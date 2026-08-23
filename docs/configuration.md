# Configuration Reference

Every configuration file and field URUFLOW reads. For how to *use* these settings, see
[Projects](projects.md) and [Operations](operations.md).

## 1. Where Configuration Lives

URUFLOW keeps each fact in exactly one place.

| Fact | Location |
| :--- | :--- |
| Server, registry and webhook settings | `/etc/uruflow/config.yaml` |
| Shared environment variables | `/etc/uruflow/defaults.yaml` |
| Project definitions | `/etc/uruflow/projects/`, or the database |
| Agent identities and keys | The database, and each agent's own config |
| Agent settings | `/etc/uruflow/agent.yaml` on the target machine |

Agents and projects are deliberately **not** in `config.yaml`.

## 2. Server — `config.yaml`

Created by `uruflow init`. Default location `/etc/uruflow/config.yaml`, overridden with `--config`
or the `URUFLOW_CONFIG` environment variable.

```yaml
server:
  host: 0.0.0.0
  ufp_port: 9001
  http_port: 9000
  advertise: uruflow.internal
  data_dir: /var/lib/uruflow

registry:
  host: uruflow.internal
  port: 5000
  namespace: uruflow
  username: uruflow
  password: <generated>
  image: registry:2
  socket: /var/run/docker.sock

webhook:
  host: 0.0.0.0
  path: /webhook
  secret: <generated>
  tls: true
  cert: /etc/letsencrypt/live/uruflow.example.com/fullchain.pem
  key: /etc/letsencrypt/live/uruflow.example.com/privkey.pem
```

### server

| Field | Default | Meaning |
| :--- | :--- | :--- |
| `host` | `0.0.0.0` | Interface both listeners bind to |
| `ufp_port` | `9001` | Where agents connect |
| `http_port` | `9000` | Where webhooks arrive |
| `advertise` | *required* | The name agents dial. Added to the server certificate. |
| `data_dir` | `/var/lib/uruflow` | Database, logs, PKI and registry storage |

`advertise` is required and validated at startup. It must be a name or address every agent can reach,
because it is placed in the server certificate that agents verify.

### registry

| Field | Default | Meaning |
| :--- | :--- | :--- |
| `host` | *required* | The name agents pull from. Added to the registry certificate. |
| `port` | `5000` | Host port the registry is published on |
| `namespace` | `uruflow` | Path segment before the project name |
| `username` | `uruflow` | Registry user |
| `password` | generated | Registry password; at least 16 characters, written to `htpasswd` with bcrypt |
| `image` | `registry:2` | Registry image to run |
| `socket` | `/var/run/docker.sock` | Docker socket the server uses to run the registry |

Images are named `<host>:<port>/<namespace>/<project>`.

`host` and `advertise` are separate fields because the registry and the agent link can be reached by
different names. Setting both to the same value is normal.

### webhook

| Field | Default | Meaning |
| :--- | :--- | :--- |
| `host` | `server.host` | Interface the webhook and health listener bind to |
| `path` | `/webhook` | Path receiving pushes |
| `secret` | generated | Shared secret for signature verification; at least 32 characters |
| `tls` | `true` | Serve the webhook and health routes over HTTPS |
| `cert` | generated server certificate | TLS certificate path; set with `key` for a publicly trusted certificate |
| `key` | generated server key | TLS private-key path; set with `cert` |

The secret is required. TLS may only be disabled when the effective webhook host is a loopback address, which is
useful when a local reverse proxy terminates HTTPS. Set `webhook.host: 127.0.0.1` to bind only this
listener to loopback while agents continue using `server.host: 0.0.0.0`.

The generated certificate is signed by URUFLOW's private CA. Public GitHub and GitLab services do not
trust that CA, so use a publicly trusted `cert` and `key`, or place an HTTPS reverse proxy in front of
a loopback-only plaintext listener.

## 3. Agent — `agent.yaml`

Written by `uruflow-agent init`. Default location `/etc/uruflow/agent.yaml` when running as root,
otherwise `~/.config/uruflow/agent.yaml`. Overridden with `--config` or the `URUFLOW_AGENT_CONFIG`
environment variable.

```yaml
agent_id: 4f2a9c1e8b3d7a60
key: <issued by the server>
roles:
  - builder
  - runner

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

| Field | Default | Meaning |
| :--- | :--- | :--- |
| `agent_id` | *required* | Identity issued by the server |
| `key` | *required* | Shared secret. Never transmitted; used to answer the challenge. |
| `roles` | `[runner]` | `builder`, `runner`, or both |
| `data_dir` | platform dependent | Source checkouts under `sources/` |
| `server.host` | *required* | Server address |
| `server.port` | `9001` | Server UFP port |
| `server.ca_cert` | `/etc/uruflow/ca.crt` | URUFLOW CA, copied from the server |
| `server.reconnect_sec` | `5` | Delay between reconnect attempts |
| `server.metrics_sec` | `10` | Metrics and container reporting interval |
| `docker.socket` | `/var/run/docker.sock` | Docker socket for workloads |

Paths default to the user's home directory when the agent runs as a non-root user, which is why a
non-root agent cannot install the registry CA into `/etc/docker/certs.d`. See
[Operations](operations.md#agent-permissions).

## 4. Project Service Specification

Services are declared under `services` in an environment YAML file. Unknown keys are rejected.

| Field | Type | Meaning |
| :--- | :--- | :--- |
| `image` | string | Immutable `repository@sha256:digest`; mutually exclusive with `dockerfile` |
| `dockerfile` | string | Build file inside the source directory |
| `context` | string | Build context inside the source directory; defaults to `.` |
| `build_args` | `map[string]string` | Docker build arguments |
| `command` | string | Container command override |
| `ports` | string list | `host:container[/protocol]` bindings |
| `volumes` | string list | `source:target[:ro]` mounts |
| `env` | `map[string]string` | Service environment merged over project environment |
| `network` | string | Docker network |
| `restart` | string | Restart policy; defaults to `unless-stopped` |
| `healthcheck` | object | Native `http`, `tcp` or `running` release readiness |
| `labels` | `map[string]string` | Generic Docker labels; `uruflow.*` is reserved |

`http` accepts `scheme`, `path`, `port`, `interval`, `timeout` and `retries`. `scheme` defaults to
`http`; timing defaults are `5s`, `3s` and `10`. `tcp` accepts `port` and the same timing fields.
`running` accepts only the required positive `stable_for` duration.

## 5. Data Directory Layout

```text
/var/lib/uruflow/
├── uruflow.db          Agents, projects, releases, logs, containers, alerts
├── uruflow.log         Server log
├── pki/
│   ├── ca.crt          Trust root every agent pins
│   ├── ca.key          Authority private key
│   ├── server.crt      Agent link certificate
│   ├── server.key
│   ├── registry.crt    Registry certificate
│   ├── registry.key
│   └── htpasswd        bcrypt registry credentials
└── registry/           Image blobs
```

Certificate lifetimes are ten years for the authority and five years for leaf certificates. Leaves are
reissued automatically when they expire or when the names they cover change — for example after
changing `advertise`.

For what to back up and why, see [Operations](operations.md#backup-and-restore).

## 6. Command Reference

### Server Commands

| Command | Effect |
| :--- | :--- |
| `uruflow` | Start the server with the terminal interface |
| `uruflow --headless` | Start without the interface |
| `uruflow init` | Write `config.yaml` |
| `uruflow agent add <name> --roles builder,runner` | Enrol an agent and print its credentials |
| `uruflow agent list` | List agents with id, roles, status and version |
| `uruflow agent remove <name>` | Remove an agent |
| `uruflow version` | Print the version |

`--config` / `-c` selects a configuration file for any of these, or set `URUFLOW_CONFIG`.

### Agent Commands

| Command | Effect |
| :--- | :--- |
| `uruflow-agent init` | Write `agent.yaml` |
| `uruflow-agent run` | Run in the foreground |
| `uruflow-agent stop` | Signal a running agent to stop |
| `uruflow-agent status` | Report whether the agent is running |
| `uruflow-agent version` | Print the version |

`uruflow-agent init` flags:

| Flag | Default | Meaning |
| :--- | :--- | :--- |
| `--id` | *required* | Agent id from the server |
| `--key` | *required* | Agent key from the server |
| `--server` | *required* | `host` or `host:port` |
| `--roles` | `runner` | Comma-separated roles |
| `--ca` | `/etc/uruflow/ca.crt` | Where the CA certificate is stored |
| `--docker-socket` | platform default | Docker socket path |

`uruflow-agent` accepts `--config` / `-c` on every subcommand, or `URUFLOW_AGENT_CONFIG` in the
environment.

## 7. Webhooks

Point your git host at `https://<server>:<http_port><webhook.path>`.

### GitHub

1. Repository → Settings → Webhooks → Add webhook
2. Payload URL: `https://your-server:9000/webhook`
3. Content type: `application/json`
4. Secret: the `webhook.secret` from `config.yaml`
5. Events: just the push event

Verified with the `X-Hub-Signature-256` header using HMAC-SHA256 over the raw body.

### GitLab

1. Project → Settings → Webhooks
2. URL: `https://your-server:9000/webhook`
3. Secret token: the `webhook.secret` from `config.yaml`
4. Trigger: push events

Verified by comparing the `X-Gitlab-Token` header against the secret.

GitHub's delivery id and GitLab's event UUID are stored for 30 days. Replayed deliveries return
`202 Accepted` without starting another release. Only push event headers are accepted.

### Routing

Pushes are matched on **git URL and branch**, not on the project name, so one repository can feed
several projects.

Every form of a git URL is treated as the same repository:

```text
git@github.com:acme/api.git
https://github.com/acme/api.git
https://github.com/acme/api
ssh://git@github.com/acme/api.git
```

A project matches when the URL resolves to the same repository, the branch equals the pushed branch,
and `auto_deploy` is true. Every match is triggered:

```json
{"repository": "api", "branch": "main",
 "triggered": [{"project": "api-prod", "release": "cf5ae9cb"}]}
```

Requests are answered `202 Accepted` whether or not a project matched. The body carries the reason
when nothing was triggered, so a git host does not record a delivery failure for a push that was
correctly ignored. Tag pushes are rejected — only `refs/heads/*` is handled.
