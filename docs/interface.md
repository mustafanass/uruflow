# Terminal Interface

The reference for operating URUFLOW day to day. For a guided first run, see
[Getting Started](getting-started.md).

The interface attaches locally to the persistent server with `sudo uruflow console` (bare
`sudo uruflow` is an alias). It owns only that terminal while it is open and needs at least 40
columns. Pressing `q` or `ctrl+c` detaches the interface; the server, agents, webhooks and releases
continue running.

## Views

```text
◆ URUFLOW   1 overview  2 projects  3 agents  4 releases  5 registry  6 alerts  7 secrets    ● 2/2 agents   ◈ uruflow.internal:5000
```

| Key | View | Shows |
| :--- | :--- | :--- |
| `1` | Overview | Fleet summary, agent resources, recent releases |
| `2` | Projects | Every project, its environment, source, and last release |
| `3` | Agents | Roles, status, resources; drill into containers and their logs |
| `4` | Releases | History and live pipelines; drill into per-runner outcome and logs |
| `5` | Registry | Repositories, tags, digests and sizes |
| `6` | Alerts | Active and resolved alerts |
| `7` | Secrets | Stored secret names, masked, with their reference form |

## Global Keys

| Key | Action |
| :--- | :--- |
| `1`–`7` | Jump to a view |
| `tab` / `shift+tab` | Cycle views |
| `↑` `↓` or `k` `j` | Move the selection |
| `enter` | Open or act on the selection |
| `esc` | Leave a form or drill-down |
| `?` | Key help and the release flow |
| `q` | Quit |
| `ctrl+c` | Quit from anywhere, including forms |

Inside a form or a drill-down, view switching is disabled — the view consumes the keys until you press
`esc`.

## Confirmations

Deploying, rolling back, stopping and every delete ask first:

| Key | Action |
| :--- | :--- |
| `y` | Confirm |
| `n` or `esc` | Cancel |

The footer names what each answer does for the action in front of you, so a `d` pressed on the wrong
row costs nothing.

## Projects

| Key | Action |
| :--- | :--- |
| `n` | New project |
| `e` | Edit the selected project |
| `enter` | Deploy |
| `r` | Roll back to the last successful image |
| `s` | Stop on every runner |
| `d` | Delete the project and its containers |
| `R` | Reload project files from disk |
| `ctrl+t` | Cycle the detail panel below the list |

The detail panel has three tabs:

| Tab | Shows |
| :--- | :--- |
| `overview` | Build settings, runners, and a compact summary of every service: source, image or Dockerfile, ports, network, healthcheck and label count |
| `variables` | Effective environment variables after merging |
| `config` | The `<env>.yaml` file, for file-backed projects |

### Create and Edit

The form has its own tabs, cycled with `ctrl+t`:

| Tab | Contents |
| :--- | :--- |
| `settings` | Fields and pickers |
| `variables` | The `.env` file — paste or type |
| `services` | Native multi-service list and editor |
| `config` | File mode only: paste `<env>.yaml` to use instead of the settings tab |

| Key | Action |
| :--- | :--- |
| `tab` / `shift+tab` | Next or previous field |
| `←` `→` | Change a picker value |
| `space` | Toggle a checkbox |
| `ctrl+t` | Next tab |
| `ctrl+s` | Validate and save |
| `esc` | Cancel; press twice to discard unsaved changes |

Fields that accept only known values are pickers rather than text: `stored as`, `builder`, `runners`
and `auto deploy`. `builder` and `runners` list only agents that hold the matching role.

In `services`, use `n` to add, `e` or `enter` to edit, and `d` to remove. The nested editor splits
settings, runtime, health timing, build arguments, service environment and labels into compact tabs.
`ctrl+s` saves the service into the project draft; save the project itself with `ctrl+s` from the
services list.

## Agents

| Key | Action |
| :--- | :--- |
| `n` | Enrol an agent |
| `d` | Remove the selected agent |
| `enter` | List that agent's managed containers |
| `enter` (on a container) | Stream its logs live |
| `esc` | Back |

The enrolment form submits with **`enter`**, not `ctrl+s`. Toggle roles with `space`.

After enrolling, URUFLOW shows the exact `uruflow-agent init` command and the path of the CA
certificate to copy. That screen is shown once — the key is not displayed again.

Selecting an agent shows its CPU, memory and disk with usage bars.

## Releases

| Key | Action |
| :--- | :--- |
| `enter` | Open the release |
| `f` | Toggle log following |
| `esc` | Back |

A release shows its stage inline:

```text
● build ── ● push ── ◉ release      building, rolling out
● build ── ● push ── ● release      live
✘ build ── – push ── ○ release      failed during build
```

Opening one shows status, image, commit, digest, per-runner outcome, and the full log with build and
release output interleaved in arrival order.

Container log following remains non-blocking. If the in-memory bridge fills, the log view reports
`▲ <count> log lines dropped` instead of stalling agent protocol event handling.

## Registry

| Key | Action |
| :--- | :--- |
| `r` | Refresh the catalog |
| `d` | Delete the selected tag |

Deleting a tag removes its manifest. It does **not** reclaim disk — see
[Operations](operations.md#garbage-collection).

## Alerts

| Key | Action |
| :--- | :--- |
| `r` | Resolve the selected alert |
| `a` | Toggle between active and all |

Alerts resolve on their own when the condition clears.

## Secrets

| Key | Action |
| :--- | :--- |
| `n` | Store a new secret |
| `d` | Remove the selected secret |

The value field is masked while typing and the value is never displayed again. The view shows each
secret's reference form, `${secret:name}`, which is what you paste into a project variable.

## Reading Status

| Symbol | Meaning |
| :--- | :--- |
| `●` | Online, running, or a completed stage |
| `○` | Offline, or a stage not started |
| `◉` | A stage in progress |
| `✔` `✘` | Succeeded, failed |
| `▲` | Warning, or an unsaved editor tab |
| `–` | Skipped or not applicable |
| `▍` | The selected row |

The header always shows how many agents are online and the registry address.
