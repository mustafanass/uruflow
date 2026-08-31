# Operations Workspace

URUFLOW is operated from one full-screen terminal workspace. The workspace owns the prompt, command
history, live output, tables, confirmations, secret entry, and inline YAML editor. Operational
commands stay inside this workspace.

The persistent server still owns live agent sessions, the pipeline, SQLite, and the registry. The
workspace reaches it through a root-only Unix control socket, so closing the interface never stops
the server or a running release. YAML files remain authoritative for project configuration.

## Open the workspace

```bash
sudo uruflow
# explicit equivalent
sudo uruflow console
```

The layout has one header, one scrollable response transcript, and one prompt.
It opens with a compact welcome card instead of immediately dumping fleet tables. Type a command and
its complete response appears in the same transcript. Long-running commands append new lines in real
time without replacing the rest of the view.

```text
status
project deploy api-prod
release logs <release-id> --follow
events
help
```

The command area filters suggestions as you type. Type `/` for the command palette or `help` for the
same list in the transcript. Use `↑` and `↓` to select, `Tab` to complete, and `Esc` to close the
list. `Enter continue` advances to the next argument; `Enter run` executes the command.

Argument choices come from the server. A container choice fills its agent and container ID together;
a registry choice fills its repository and tag. The stage line shows the current position, for
example `DEPLOY › PROJECT › MODE`. Loading, empty, and failed requests are shown in the command area.
When the prompt is empty, `↑` and `↓` browse saved non-sensitive history.

Use `PgUp` and `PgDn` to inspect the transcript, `Ctrl+L` or `clear` to clear it, and `Ctrl+D` or
`exit` to close the workspace. New stream lines do not pull the view away while you are reading older output.
`Ctrl+C` detaches the visible live stream without cancelling a durable server-side release.

Set `NO_COLOR=1` or open with `uruflow --no-color` when ANSI color is not wanted.
The palette uses the URUFLOW navy, gold, ivory, and steel-blue colors. Green and red are reserved for
health and failures.

## Status and live activity

```text
status
events
```

`status` returns the fleet summary, agents, and projects. `events` follows releases, build output,
state changes, agent connectivity, and alerts. A new stream starts at the current sequence. Detaching
prints a cursor that can resume the stream:

```text
events --after 142
```

The server keeps the latest 8192 entries in memory. If the requested sequence is older, the workspace
reports the gap and resumes from the oldest retained entry.

## Creation and guided input

Input depends on the value:

- Short values such as a new agent name stay in the command box with a labeled next-step prompt.
- Existing resources such as projects, agents, and releases are selected from daemon-backed choices.
- Secret values switch to masked entry and never enter command history.
- Project YAML uses the multiline editor because it benefits from paste, line numbers, and validation.

`project deploy` does not create configuration: it operates on an already loaded project. Use `project
create PROJECT ENV` for a new project, or manage the same authoritative files with any external
editor. Once loaded, `project deploy ` offers the project.

## Projects and YAML

```text
project list
project create api prod
project show api-prod
project edit api-prod
project validate /etc/uruflow/projects/api/prod.yaml
project apply api prod /etc/uruflow/projects/api/prod.yaml
project reload
```

`project edit` temporarily opens the authoritative environment file in `$VISUAL`, then `$EDITOR`,
falling back to `vi`. The workspace returns to the same page and reloads YAML when the editor exits.

`project create api prod` opens the full-width project editor with a project, environment, and service
template. It supports paste, scrolling, line numbers, automatic YAML indentation, `Tab` and
`Shift+Tab`. Press `Ctrl+S` to format, validate, write both project files, and reload them. A validation
error returns to the editor without losing the text.

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

For container output, type `container logs ` and select a row. The choice fills `AGENT CONTAINER`,
then offers recent output, fixed tail sizes, or live follow. Registry rows are marked `system` and
project workloads are marked `service`. A full local log buffer pauses the producer instead of
silently dropping lines.

Deploy and rollback follow their process by default. Reattach to durable work later with `release
follow`. Container logs stream over the existing agent connection; no extra SSH session is opened.

`secret set` changes the prompt to masked entry. The value is never put in the command line,
transcript, or history. Removing agents, stopping projects, deleting registry manifests, resolving
alerts, and removing secrets require an in-page confirmation.

Text received from agents, containers, projects, and backend errors is sanitized before rendering.
Only ANSI styling produced by URUFLOW reaches the terminal.

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

Files describe desired state. The database records observed state and history. The workspace does not
add another configuration source.
