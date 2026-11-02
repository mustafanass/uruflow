# Browser workspace

The browser workspace is the operator interface for URUFLOW. Its pages share live observations, project scope and the current user's permissions.

## Navigation

| Page     | Work                                                                                  |
| -------- | ------------------------------------------------------------------------------------- |
| Overview | Check fleet status, open active alerts and explore the live infrastructure graph      |
| Projects | Manage project environments and follow releases across the fleet                      |
| Agents   | Enroll machines, inspect host resources and browse container inventory                |
| Activity | Follow deployment, connectivity and alert events; inspect alert history               |
| System   | Inspect the server and registry, manage access and review the administrator audit log |

Use **Ctrl/⌘ K** to find projects, agents, containers and releases directly. Resource detail URLs remain stable. Existing index links for Infrastructure, Releases, Containers, Alerts and Registry open their new locations.

Tab selection and table filters stay in the URL, including when you move between views. Your account menu is under the avatar in the sidebar. It provides password changes, theme selection, sign-out and sign-out everywhere. The top bar keeps resource search, connection status and help.

## Overview and graphs

Overview combines a fleet summary, the attention list and the live infrastructure workspace. Active alerts open on the same page. Container and release totals lead to their fleet views under Agents and Projects.

Fleet graphs connect the server to its agents, then agents to their projects. Build links and assigned or observed runtime placements remain distinct. Projects without a host stay visible as unconnected cards. **Show containers** expands a host's observed workloads; search can find a container while its host is collapsed or show the hosts assigned to a project.

Cards show state, service readiness, host and container CPU and memory, container uptime and restarts, or the project's latest release. Missing samples remain distinct from zero usage. Measurements update without moving the graph. The canvas uses the available screen height, and **Expand graph** provides more room. Small graphs omit the minimap.

Two-finger trackpad scrolling pans horizontally and vertically. Pinch, **Ctrl/⌘ + scroll** or the zoom buttons change the scale. Dragging the canvas also pans the view. Fit and reset share the same framing; tall project graphs retain a readable scale and can be panned.

Select a node to open its inspector without changing zoom or pan. Click the canvas or use its close control to dismiss it. Dragged positions save automatically for that account and graph in the browser. Existing saved positions migrate when card dimensions change. A storage failure leaves a visible **Retry save** control. Reset returns to automatic placement. Layout changes do not block deployment or navigation.

## Projects

The project directory also provides fleet release history. Each project has **Overview**, **Graph**, **Releases**, **Configuration** and **Activity**, according to permissions.

Overview groups observed instances under their services. Each instance shows its runner, CPU, memory and a direct log link. Published ports and dependencies sit with the service they belong to. Latest-release information and assigned-machine resources appear alongside them. **Resource trends** opens session charts without leaving the page.

Graph retains runtime and delivery views. Runtime connects service intent, runner placement and observed containers. Delivery shows build and release relationships. The inspector includes service definitions, limits, dependencies, storage and observed instances.

**Deploy** starts a release of the saved configuration. **Rollback** selects an eligible successful release. **Stop** stops managed project containers. Actions are disabled when their required state is absent or another release owns the project.

Configuration contains the authoritative YAML and variable overlay as separate editors. Each editor has its own revision, validation, save control and unsaved state. Saving one preserves unsaved edits in the other. Both contribute to the navigation guard. Editors remain read-only while their own save is in flight.

If a save conflicts with another revision, use **Copy unsaved edits** if needed, then **Load latest**. Confirming loads the new file and revision so you can apply changes again. During a conflict, **Discard edits** uses the same recovery path.

Saving does not deploy. [Projects](projects.md) owns file layout and variable syntax.

## Releases

A release has **Overview**, **Logs** and **Changes**. Active releases open on Logs. Selecting another tab keeps that selection while the release progresses.

Overview brings together progress, the release record, runner outcomes and immutable artifacts. Changes compares the saved definition with the preceding release for the same project. A first release shows its initial definition. The saved definition remains available for inspection and copying; variable values are redacted.

## Agents and containers

The Agents directory provides machine inventory and a fleet Containers view. An agent has **Overview**, **Workloads** and **Metrics**. Workloads groups its assigned projects, observed containers and build work. Administrators remove an agent from the danger section in Overview after unassigning its projects.

A container has **Overview**, **Logs**, **Metrics** and **Inspect**. Overview brings together identity, state, ports, networks, storage and health probes. Direct project and host links keep related resources within reach.

Enrollment displays its setup command once. Save it, then confirm **I saved the enrollment command** before closing an unconnected agent's setup. The dialog also closes after the agent connects. The CA download remains available on Agents.

## Activity and System

Activity combines the live event feed with release records and durable alert history. Filter the timeline by event type or resource. The Alerts view includes active and resolved conditions. Operators can resolve alerts within their project scope.

System groups **Server**, **Access** and **Audit log**. Server includes host resources and the registry catalogue. Fleet access is required for registry data. Access and Audit log are administrator-only; shared Activity does not expose administrative audit records.

The live event feed is a bounded server buffer. Release records, alert history and the administrative audit use persistent storage. [Security](security.md) explains roles, project grants and the local account recovery command.

## Live state and logs

The workspace shows connection state and marks old observations visibly. A reconnect preserves the last observation while the browser restores the stream. Empty fleets identify that no agents are enrolled.

Release logs are stored on the server. Container logs are collected from Docker through an agent. The viewer supports search, wrapping, follow controls, reconnect and download. Hidden browser tabs pause their log connection. Returning or reconnecting resumes from the last delivered cursor without appending the same tail again. Completed streams hide follow controls. `stderr` identifies an output stream; it does not classify every line as an error.

When a container leaves the inventory, its open detail view retains the last observation and loaded logs. Use **Download loaded logs** to keep that browser buffer before leaving the view.

Log output is buffered in batches and retains the latest 2,000 loaded lines. Reconnection resumes each Docker stream from its saved cursor. Removed containers, invalid cursors and stream limits show an actionable error with manual retry. Relative times update independently of workspace snapshots, and hidden tabs pause time updates and registry polling.

The system, light and dark theme preference is saved in the browser and shared across tabs. Page transitions and loading indicators respect reduced-motion preferences. Arrow keys navigate tabs and the account menu; **Esc** closes dismissible surfaces. Narrow screens provide a navigation menu and scrollable page tabs.

Related: [Getting started](getting-started.md), [Configuration](configuration.md), [Troubleshooting](troubleshooting.md).
