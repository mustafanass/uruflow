# Getting started

This guide takes a new installation from a source build to its first release. The server hosts the browser workspace and registry; agents build or run applications on their own machines.

## Prepare the machines

For a source build, install the Go version in [go.mod](../go.mod), Node.js 24, npm and a C compiler for SQLite. The server needs Docker Engine and access to its socket. Builder agents also need Git, the Docker CLI and access to their source repositories. Runner agents need Docker Engine and socket access.

Choose a DNS name or address reachable by all agents, such as `uruflow.internal`. Use the same release of the server and agent binaries.

## Build and initialize the server

From the repository root:

```sh
make build
sudo install -m 0755 build/uruflow /usr/local/bin/uruflow
sudo uruflow init --advertise uruflow.internal
sudo uruflow serve
```

`init` writes `/etc/uruflow/config.yaml` and creates its project directory. `serve` opens the database, prepares PKI, starts the registry and listeners, and serves the embedded frontend. Keep it running while completing setup.

For service installation, use [Operations](operations.md#services). Configuration paths and listener defaults are in [Configuration](configuration.md).

## Create the first administrator

On the server, open `http://127.0.0.1:9002`. For remote use, configure the operator HTTPS address first, following [Configuration](configuration.md#operator-listener).

Read the setup token on the server:

```sh
sudo cat /var/lib/uruflow/web-setup-token
```

Enter the token, choose an administrator username and password, and finish setup. Then sign in. The server consumes the token after creating the first administrator. Add other accounts and project grants in **System → Access**.

## Enroll an agent

1. Open **Agents → Enroll agent** and choose a name and roles.
2. Install `build/uruflow-agent` as `/usr/local/bin/uruflow-agent` on the target machine.
3. Run the generated initialization command on that machine.
4. Download the CA certificate and install it at the path shown by setup, normally `/etc/uruflow/ca.crt`.
5. Start the agent:

```sh
sudo uruflow-agent run
```

Save the enrollment command before closing setup. The CA certificate also remains available from **Agents**. Wait for the agent to show **online** with a recent sample. Repeat for the builders and runners you need.

## Create a project

Open **Projects → Create project**. Choose a project name, environment and service source.

For source builds, select a builder and enter the repository, branch and Dockerfile. For prebuilt services, supply a full `repository@sha256:digest` reference. Select runner agents for deployment workflows.

The project opens on its **Overview** tab. Review services, ports, placement and variables. Use the YAML and variables editors in **Configuration** for the definition and plain or protected values. [Projects](projects.md) explains both files.

## Deploy and inspect

Choose **Deploy**. The release page shows progress, artifacts, runner outcomes and logs. Follow the result there; starting a release and completing it are separate events.

A project's **Overview** is its runtime or delivery graph. The fleet graph is on **Overview**. Open a container to inspect its resources, networking, storage, health and logs. A release continues when you close the browser.

After a successful release, the project history provides artifacts for a later restore. See [Deployments](deployments.md) for readiness, replacement and rollback.

Related: [Browser workspace](interface.md), [Operations](operations.md), [Troubleshooting](troubleshooting.md).
