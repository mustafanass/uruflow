# Troubleshooting

Symptoms, causes and fixes. Start with the log — the server writes to `<data_dir>/uruflow.log` and
each agent to its own `log_file`.

## Server Will Not Start

**`registry.host is required` / `server.advertise is required`**

Both are mandatory and validated before anything starts. They are placed in the certificates agents
verify, so URUFLOW will not guess them. Set both in `config.yaml`.

**`uruflow needs docker to host the registry`**

The server could not reach the Docker socket at `registry.socket`. Check the daemon is running and the
user can use it.

**`registry did not become healthy on <host>`**

The registry container started but did not answer within 60 seconds. Check `docker logs
uruflow-registry`. Common causes: the published port is already in use, or `<data_dir>/registry` is
not writable.

On macOS, port 5000 is often taken by AirPlay Receiver. Choose another `registry.port`.

## Agent Will Not Connect

Read the agent log first; the reason is almost always stated there.

**`agent not registered or invalid key`**

The `agent_id` or `key` in `agent.yaml` does not match the server's record. Re-enrol with
`uruflow agent add` and rewrite the config with the printed command.

**`x509: certificate signed by unknown authority`**

The agent is not using the server's CA. Copy it again:

```bash
scp <server>:/var/lib/uruflow/pki/ca.crt /tmp/ca.crt
sudo mv /tmp/ca.crt /etc/uruflow/ca.crt
```

**`x509: certificate is valid for …, not uruflow-server`**

The server certificate does not carry the expected logical name. This happens if the PKI directory was
partly restored. Delete `pki/server.crt` and `pki/server.key` and restart the server — they are
reissued from the CA.

**`agent declared no valid role`**

`roles:` is empty or misspelled. It must contain `builder`, `runner`, or both.

**Connection refused**

Check `server.host` in `config.yaml` binds an interface the agent can reach, that `ufp_port` matches,
and that nothing between them blocks the port. Agents always dial outward, so no inbound rule is needed
on the agent side.

## Releases

**`a release is already running for this project`**

Exactly what it says. One release per project at a time. Wait for it, or check the releases view for a
release stuck because an agent vanished — a server restart closes those out.

**`builder agent is offline` / `project has no runner agents`**

The configured builder or runners are not connected, or do not hold the role. Check `uruflow agent
list`.

**Build fails with `failed to fetch anonymous token` or a DNS timeout**

The builder could not reach the base image registry. This is the builder's network, not URUFLOW.
Base images are cached after the first successful pull.

**A prebuilt service fails to pull with `unauthorized`**

Credentials are only sent to the URUFLOW registry, so a public image is pulled anonymously. This error
from a public registry means the image name is wrong, the tag does not exist, or the runner has no
outbound access — not that authentication is misconfigured.

**Push fails with a certificate error**

The builder's Docker daemon does not trust the URUFLOW CA. The agent installs it automatically at
`/etc/docker/certs.d/<registry>/ca.crt`, but that needs root. Check the agent log for `could not
install the registry CA` and run the agent as root.

**Release succeeded but the container is not running**

Check readiness semantics in [Deployments](deployments.md#readiness). A release only succeeds after
the container is ready, so this usually means it exited afterwards. Inspect its logs from the agents
view, or `docker logs uruflow-<project>`.

**Release failed and the old version is still running**

That is the intended behaviour. The replacement failed readiness and the previous container was
restored. The release message and log say why.

**One runner failed, others succeeded**

The release is marked failed and the fleet is split — successful runners are on the new image, the
failed one kept its previous container. URUFLOW does not roll the others back. Fix the failing runner
and deploy again, or roll back deliberately.

**Release stuck in `building` or `releasing`**

Only possible while the server is down. Restarting it closes out interrupted releases as failed.

## Containers

**A container is not managed by URUFLOW**

URUFLOW only sees containers labelled `uruflow.managed=true`. Containers started by hand are invisible
and are never touched.

**A `-previous` container is left behind**

A set-aside container is removed once its replacement is ready, or renamed back if it is not. One
surviving after a crash mid-release is safe to remove by hand:

```bash
docker rm -f uruflow-<project>-previous
```

**Container logs show nothing**

The stream is working; the container is not writing to stdout. Applications that log to a file inside
the container produce nothing here. The empty state distinguishes the two cases.

## Secrets

**`references "x": secret is not set`**

A project variable references a secret that does not exist. The deploy is refused before any build
starts. Press `7` in the interface and store it.

**A secret cannot be read back**

By design. The interface shows names and masks only. To change one, store it again under the same
name.

**`secret material is malformed` after a restore**

`pki/secrets.key` does not match the ciphertext in the database — usually a database restored without
its key, or the reverse. Both must come from the same backup. If the key is lost, every secret must be
set again from its original source.

## Projects and Files

**`field brnach not found in type projects.Environment`**

A misspelled key. Unknown fields are rejected by name rather than being silently ignored.

**`unknown agent "web-99"` / `does not carry the builder role`**

The file names an agent that does not exist, or one without the required role. Names come from
`uruflow agent list`.

**`came from this file, which is gone`**

A file-backed project's file was deleted. The project keeps running; press `d` to remove it, or restore
the file and press `R`.

**Edits to project files have no effect**

Files are read on start and when you press `R`. There is no filesystem watcher.

**Editing a project lost a field**

The settings form owns a fixed set of fields. `build_args`, `command`, `restart` and project-level
`env` are preserved but can only be *changed* by editing the file — through the `config` tab or on
disk. Comments in YAML are lost when the settings form writes a file back.

## Webhooks

**Pushes do nothing**

Check, in order: the project's branch matches the pushed branch; `auto_deploy` is true; the git URL in
the project resolves to the same repository as the push. The response body states which condition
failed.

**`invalid signature`**

The secret in your git host does not match `webhook.secret`. GitHub uses `X-Hub-Signature-256`, GitLab
uses `X-Gitlab-Token` — configuring the wrong one for the host gives this error.

**Tag pushes are ignored**

Only `refs/heads/*` is handled.

## Terminal Interface

Keys and views are listed in [Terminal Interface](interface.md).

**The display is corrupted**

The interface owns the terminal while it runs. Nothing else should write to it. If the server is also
running headless in the same terminal, stop that first.

**Changes are not visible after an upgrade**

A running process keeps its old code. Quit with `q` and start it again.

**`terminal too narrow`**

The interface needs at least 40 columns.

## Getting More Detail

The server log records every release transition, agent connection and file-loading problem. Failed
releases keep their full build output in the releases view — open the release and read the log rather
than guessing from the status.
