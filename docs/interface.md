# Operations Workspace

URUFLOW is operated from one full-screen terminal workspace. The workspace owns the prompt, command
history, live output, tables, confirmations, secret entry, and inline YAML editor. Operational
commands are deliberately not duplicated as external shell subcommands.

The persistent server still owns live agent sessions, the pipeline, SQLite, and the registry. The
workspace reaches it through a root-only Unix control socket, so closing the interface never stops
the server or a running release. YAML files remain authoritative for project configuration.

## Open the workspace

```bash
sudo uruflow
# explicit equivalent
sudo uruflow console
```

The layout stays intentionally small: one header, one scrollable response transcript, and one prompt.
It opens with a compact welcome card instead of immediately dumping fleet tables. Type a command and
its complete response appears in the same transcript. Long-running commands append new lines in real
time without replacing the rest of the view.

```text
status
deploy api-prod
logs <release-id> --follow
events
help
```

The command area filters suggestions as each character is typed and displays a short explanation for
every match. Type `/` to open the complete command palette or `show`/`help` to print the complete
reference in the transcript. Use `↑` and `↓` to select, `Tab` to complete, `Enter` to run, and `Esc`
to close suggestions. When the prompt is empty, `↑` and `↓` browse history.

Completion is contextual rather than a flat list. `project ` shows project subcommands; `deploy `,
`rollback `, and `stop ` query the daemon and show actual loaded projects. Agent inspection/removal
shows actual agents, while release inspection/log commands show real release IDs. Selecting a command
that needs an argument advances to that argument instead of repeatedly offering the same command.

`deploy`, `rollback`, and `logs` are short forms for their namespaced commands. Use `PgUp` and `PgDn`
to inspect the transcript, `Ctrl+L` or `clear` to clear it, and `Ctrl+D`, `quit`, or `exit` to close
the workspace. New stream lines do not pull the view away while you are reading older output.
`Ctrl+C` detaches the visible live stream without cancelling a durable server-side release.

Set `NO_COLOR=1` or open with `uruflow --no-color` when ANSI color is not wanted.

## Status and live activity

```text
status
events
```

`status` returns a compact fleet card followed by agent and project tables. `events` behaves like a
fleet-wide `tail -f`: it follows new releases, build output, state changes, and alerts until detached.
A new stream starts at the present; it does not replay every line from completed historical releases.
Starting it clears the ordinary transcript and opens a focused live-activity page with timestamped,
source-labelled output.

## Creation and guided input

The input surface matches the size and sensitivity of the value:

- Short values such as a new agent name stay in the command box with a labeled next-step prompt.
- Existing resources such as projects, agents, and releases are selected from daemon-backed choices.
- Secret values switch to masked entry and never enter command history.
- Project YAML uses the multiline editor because it benefits from paste, line numbers, and validation.

`deploy` does not have a `create` subcommand: it operates on an already loaded project. Create or edit
the authoritative project YAML, run `project reload`, and then `deploy ` will offer that project.

## Projects and YAML

```text
project list
project show api-prod
project edit api-prod
project validate /etc/uruflow/projects/api/prod.yaml
project apply api prod /etc/uruflow/projects/api/prod.yaml
project reload
```

`project edit` temporarily opens the authoritative environment file in `$VISUAL`, then `$EDITOR`,
falling back to `vi`. The workspace returns to the same page and reloads YAML when the editor exits.

To keep the whole edit inside the workspace, use:

```text
project apply api prod -
```

The prompt becomes a multiline YAML editor. Paste or write the document, press `Ctrl+S` to validate
and apply it, or `Esc` to cancel. URUFLOW writes through a temporary file and atomically replaces the
environment file. If full project validation fails, it restores the previous file.

Project definitions remain normal files at `projects/<name>/project.yaml`, so they work naturally
with Git, any editor, and configuration management. The interface never creates a competing project
definition in SQLite.

## Command reference

These commands are typed at the workspace prompt, without a leading `uruflow`:

```text
agent list
agent show builder-01
agent add builder-01 --roles builder,runner
agent remove builder-01

container list
container logs web-01 <container-id> --tail 200
container logs web-01 <container-id> --follow

project deploy api-prod
project deploy api-prod --no-follow
project rollback api-prod
project stop api-prod

release list --limit 50
release show <release-id>
release logs <release-id>
release logs <release-id> --follow
release follow <release-id>

registry list
registry remove <repository> <tag>
alert list
alert resolve <id>
secret list
secret set api_db
secret remove api_db
```

For guided enrollment, type `agent add NAME`. As soon as the name is present, the command area offers
`runner`, `builder`, and `builder,runner`; selecting one completes the command. The result card prints
the one-time agent initialization command and the server trust-root path.

Deploy and rollback follow their process by default. Reattach to durable work later with `release
follow`. Container logs stream over the existing agent connection; no extra SSH session is opened.

`secret set` changes the prompt to masked entry. The value is never put in the command line,
transcript, or history. Removing agents, stopping projects, deleting registry manifests, and removing
secrets require an in-page confirmation.

## External lifecycle commands

Only process lifecycle and setup stay outside the workspace:

| Shell command | Purpose |
| :--- | :--- |
| `uruflow` | Open the operations workspace |
| `uruflow console` | Open it explicitly |
| `uruflow serve` | Run the persistent server for systemd |
| `uruflow init --advertise <host>` | Create server YAML and credentials |
| `uruflow version` | Print the installed version |

## Configuration ownership

| Data | Authority |
| :--- | :--- |
| Server and registry settings | `config.yaml` |
| Project definitions | `projects/<name>/project.yaml` |
| Environment definitions | `projects/<name>/<env>.yaml` |
| Environment overlays | `.env` files |
| Agent local settings | `agent.yaml` |
| Enrollment, releases, logs, metrics, alerts, and secrets | SQLite/runtime state |

Files describe desired state; the database records observed state and history. The workspace is a
controlled view over those systems, not another source of truth.
