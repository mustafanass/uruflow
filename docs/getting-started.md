# Getting Started

From nothing to a running deployment and a rollback. You do not need to read the architecture or
protocol documents first.

Commands are marked with where they run:

- **[server]** — the machine running the URUFLOW Server
- **[target]** — a machine that will build or run workloads

For a single-machine setup, both are the same box.

## 1. Prerequisites

| Machine | Needs |
| :--- | :--- |
| Server | Docker, root, a reachable hostname or IP, free ports for the agent link, webhooks and the registry |
| Builder | Docker, `git`, and root |
| Runner | Docker |

The server runs as root: it writes `/etc/uruflow` and `/var/lib/uruflow`, and drives the Docker socket
to host the registry. A builder needs root as well, to install the registry certificate and run
`docker login`. A runner needs only access to the Docker socket.

The server always reads `/etc/uruflow/config.yaml`. The agent's default path follows the user: root
uses `/etc/uruflow/agent.yaml`, anyone else `~/.config/uruflow/agent.yaml`. This guide runs everything
with `sudo` so the paths match what it shows.

## 2. Install and Initialise the Server

**[server]**

```bash
curl -fsSL https://github.com/mustafanass/uruflow/releases/latest/download/uruflow-linux-amd64 -o uruflow
chmod +x uruflow && sudo mv uruflow /usr/local/bin/

sudo uruflow init --advertise uruflow.internal
```

`init` writes YAML using secure generated registry and webhook credentials:

| Flag | Meaning |
| :--- | :--- |
| `--advertise` | The name agents dial. It is required and enters the server certificate. |
| `--registry-host` | The name agents pull from. Defaults to `--advertise`. |
| `--data-dir` | Runtime state location. Defaults to `/var/lib/uruflow`. |

Use a name every agent can reach. `localhost` works only if everything runs on one machine.

## 3. Start URUFLOW

**[server]**

```bash
sudo install -m 0644 packaging/systemd/uruflow.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now uruflow
sudo uruflow
```

On first start it generates a certificate authority, issues certificates for itself and the registry,
and starts `registry:2` with TLS and authentication. Bare `uruflow` opens the operations workspace;
type `status` there to query the running service. If you installed only the binary, copy the unit from
[Operations](operations.md#systemd) first.

Confirm the registry reports healthy before continuing.

## 4. Enrol an Agent

**[server]** — for a single machine, enrol both builder and runner roles.

URUFLOW prints the exact command to run on the target. Type this in the workspace:

```text
agent add builder-01 --roles builder,runner
```

```text
enrolled builder-01

run this on the target machine:

  uruflow-agent init \
    --id a05fb0c7a59d9dee \
    --key 4a73594bd77adb63c46d64248667da98… \
    --server uruflow.internal:9001 \
    --roles builder,runner

then copy the trust root:

  scp /var/lib/uruflow/pki/ca.crt <host>:/etc/uruflow/ca.crt
```

The printed `scp` line assumes you can write `/etc/uruflow` on the target. If you cannot, use the
two-step copy in the next section instead.

## 5. Install the Agent

**[target]**

```bash
curl -fsSL https://github.com/mustafanass/uruflow/releases/latest/download/uruflow-agent-linux-amd64 -o uruflow-agent
chmod +x uruflow-agent && sudo mv uruflow-agent /usr/local/bin/

sudo uruflow-agent init \
  --id     <agent-id> \
  --key    <agent-key> \
  --server uruflow.internal:9001 \
  --roles  builder,runner
```

`init` creates `/etc/uruflow`, owned by root. Copy the trust root into it in two steps, then install
and start the agent service:

```bash
scp <server>:/var/lib/uruflow/pki/ca.crt /tmp/ca.crt
sudo mv /tmp/ca.crt /etc/uruflow/ca.crt

sudo install -m 0644 packaging/systemd/uruflow-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now uruflow-agent
```

If the target has only the released agent binary, copy the agent unit from
[Operations](operations.md#systemd). For a temporary foreground test, `sudo uruflow-agent run` still
works and now prints the same lines it saves to `/var/log/uruflow-agent.log`.

Within a few seconds, `agent list` in the workspace shows it **online** with its roles. If it does not, the
agent log states why — see [Troubleshooting](troubleshooting.md#agent-will-not-connect).

Run a builder as root. Without it the registry certificate cannot be installed and pushes fail with a
certificate error.

## 6. Create a Project

**[server]** — create `/etc/uruflow/projects/demo/project.yaml` and `prod.yaml` using the examples in
[Projects](projects.md#3-file-layout). Use `builder-01` as both builder and runner for this
single-machine example, then validate and load the files:

```text
project validate /etc/uruflow/projects/demo/prod.yaml
project reload
project show demo-prod
```

The loader rejects unknown agents, role mismatches, invalid workflows and malformed service models.

## 7. Deploy

Start the deployment in the workspace. Its response follows live output until completion:

```text
project deploy demo-prod
```

The process updates as it moves:

```text
◉ build ── ○ push ── ○ release      cloning and building
● build ── ● push ── ◉ release      pushed, rolling out
● build ── ● push ── ● release      live
```

Use `release list` and `release follow <id>` to inspect or reattach later.

## 8. Verify

**[server]** — `container list` in the workspace should show the project container as `running`, and
`healthy` if the image declares a `HEALTHCHECK`. Stream application output with `container logs
builder-01 <container-id> --follow`.

From a shell:

```bash
curl http://<runner>:8080/
docker ps --filter label=uruflow.managed=true
```

## 9. Roll Back

Deploy a second time so there is history, then run:

```text
project rollback demo-prod
```

Rollback re-releases the image from the last successful release. It does not rebuild, so what returns
is byte-identical to what ran before.

```text
r2  succeeded  demo@sha256:8f2a…   ← current
r1  succeeded  demo@sha256:1c9d…   ← rollback releases this
```

Rollback fails with `no successful release to roll back to` if the project has never had one.

## 10. Deploy on Push

Add a webhook at your git host pointing to `https://<server>:9000/webhook`, using the `webhook.secret`
from `config.yaml`. Pushes to a project's branch then deploy it automatically.

See [Configuration](configuration.md#6-webhooks) for GitHub and GitLab setup and how pushes are matched.

## Where to Next

| You want to | Read |
| :--- | :--- |
| Run several environments of one service | [Projects](projects.md) |
| Understand what happens during a release | [Deployments](deployments.md) |
| Run this as a service and back it up | [Operations](operations.md) |
| Evaluate the trust model | [Security](security.md) |
| Understand or change the code | [Architecture](architecture.md) |
| Look up a key or symbol | [Terminal Interface](interface.md) |
