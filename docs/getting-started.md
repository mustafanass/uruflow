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

sudo uruflow init
```

`init` asks for four things:

| Prompt | Meaning |
| :--- | :--- |
| server host | The name agents will dial. Goes into the server certificate. |
| registry host | The name agents will pull from. Defaults to the server host. |
| ufp port / webhook port | Defaults 9001 and 9000 |
| data dir | Defaults `/var/lib/uruflow` |

Use a name every agent can reach. `localhost` works only if everything runs on one machine.

## 3. Start URUFLOW

**[server]**

```bash
sudo install -m 0644 packaging/systemd/uruflow.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now uruflow
sudo uruflow console
```

On first start it generates a certificate authority, issues certificates for itself and the registry,
and starts `registry:2` with TLS and authentication. `uruflow console` then attaches the interface to
that running service. If you installed only the binary rather than a source checkout, copy the unit
from [Operations](operations.md#systemd) first.

Press `5` to confirm the registry reports healthy before continuing.

## 4. Enrol an Agent

**[server]** — press `3`, then `n`. Type a name, `tab` to the roles field, toggle roles with `space`,
then press `enter`.

> The agent form submits with `enter`; the project form submits with `ctrl+s`. They differ today.

For a single machine, tick both `builder` and `runner`.

URUFLOW prints the exact command to run on the target. The same is available from the shell:

```bash
sudo uruflow agent add builder-01 --roles builder,runner
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

Within a few seconds the agents view shows it **online** with its roles. If it does not, the agent log
states why — see [Troubleshooting](troubleshooting.md#agent-will-not-connect).

Run a builder as root. Without it the registry certificate cannot be installed and pushes fail with a
certificate error.

## 6. Create a Project

**[server]** — press `2`, then `n`.

The first field asks where the project lives. Choose **standalone** for now; you can move to files
later without changing anything else.

| Field | Example |
| :--- | :--- |
| name | `demo` |
| git url | `git@github.com:acme/demo.git` |
| branch | `main` |
| dockerfile | `Dockerfile` |
| context | `.` |
| builder | pick `builder-01` |
| runners | tick `builder-01` |
| ports | `8080:80` |
| auto deploy | `yes` |

Press `ctrl+t` to reach the `variables` tab and paste a `.env` if your application needs one. Press
`ctrl+s` to save.

Agent names are picked from a list, so a project cannot reference an agent that does not exist.

## 7. Deploy

Select the project and press `enter`, then confirm.

Press `4` to watch it. The pipeline updates as it moves:

```text
◉ build ── ○ push ── ○ release      cloning and building
● build ── ● push ── ◉ release      pushed, rolling out
● build ── ● push ── ● release      live
```

Press `enter` on the release to read the full build output as it streams.

## 8. Verify

**[server]** — press `3`, select the agent, press `enter` to list its containers. The project's
container should be `running`, and `healthy` if the image declares a `HEALTHCHECK`.

Press `enter` again on a container to stream its logs live.

From a shell:

```bash
curl http://<runner>:8080/
docker ps --filter label=uruflow.managed=true
```

## 9. Roll Back

Deploy a second time so there is history, then select the project and press `r`.

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
