# Upgrading to URUFLOW 3

URUFLOW 3.0.2 uses the browser for operator work and keeps TLS UFP for server-agent communication. Build and install the server, agents and embedded frontend from the same release.

## Prepare

1. Wait for active releases to finish.
2. Record the current configuration paths and versions.
3. Take a coordinated [backup](operations.md#backup-and-restore) of configuration, project files, database, PKI and registry data.
4. Build and validate the new release:

```sh
make check
make build
build/uruflow version
build/uruflow-agent version
```

Both commands should report `3.0.2`. The frontend package uses the same release version; the running workspace displays the server's reported version.

## Replace a current UFP installation

1. Stop the server service.
2. Install the new `build/uruflow` binary and start the server with `uruflow serve` and the existing configuration.
3. Replace and restart each builder and runner agent with the matching `build/uruflow-agent` binary.
4. Confirm recent agent samples and registry readiness before deploying.

SQLite creates the current tables and adds missing columns when opened. Preserve the existing project definitions, CA and encryption key. Server startup closes interrupted releases as failed and refreshes observations as agents reconnect.

URUFLOW 3 keeps UFP frame version `0x04`. Update agents for independent browser log streams and completion messages. The product major version does not renumber the protocol byte.

## Move operator work to the browser

Configure the separate `web` listener, default `127.0.0.1:9002`. Use [Configuration](configuration.md#operator-listener) for HTTPS or a local reverse proxy.

For an installation without browser users, use the private setup token to create its first administrator. For an existing browser installation, keep the account database and sign in normally. Review user roles and project grants under **System → Access**.

Project editing, deployment, restoration, agent enrollment, observations and logs now use the [browser workspace](interface.md). The `uruflow` command without arguments prints help. Services must invoke `uruflow serve`; agent services invoke `uruflow-agent run`.

## Validate the installation

Check sign-in, account scope, agent connectivity, registry access, project definitions, historical release details and container observations. Verify a safe deployment and restore in staging before routine production changes.

Valid persisted browser sessions survive later server restarts, and streams reconnect. Fresh server and agent reports should show `3.0.2`.

## Earlier protocol generations

Installations using a UFP frame version before `0x04` need coordinated server and agent replacement. Review their project definitions against the current [model](native-build-model.md), including service-owned sources and pinned images, before starting releases.

A 1.x installation uses a different deployment and storage model. Prepare a fresh configuration and enrollment, recreate the desired project files, and perform a first release to populate its new history. Keep the original installation backup during migration.

Related: [Getting started](getting-started.md), [Operations](operations.md), [UFP](protocol.md), [Testing](testing.md).
