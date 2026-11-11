# Testing

URUFLOW keeps tests with the package that owns behavior. The browser migration preserves the existing approach: a deterministic default gate, explicit infrastructure lanes, race checks for concurrency, and a separate visual preview.

## Default validation pass

Install the Go version in [go.mod](../go.mod), a C compiler, Node.js 24 and npm. From the repository root:

```sh
make frontend-deps
make fmt
make check
make build
```

Run the pass after implementation. Fix failures before repeating the affected checks. CI runs the same `make check` and `make build` commands.

| Command      | Checks                                                                                                  |
| ------------ | ------------------------------------------------------------------------------------------------------- |
| `make fmt`   | Format Go and frontend files                                                                            |
| `make test`  | Deterministic Go and frontend behavior tests                                                            |
| `make check` | Production frontend build, then Go formatting, vet and tests; TypeScript, frontend tests and formatting |
| `make build` | Production frontend plus server and agent binaries                                                      |

Dependency installation may fetch packages. The deterministic tests themselves must not require Docker, external network access, root, fixed ports or operator input. Temporary files, SQLite databases and test-assigned loopback listeners are allowed.

## Test ownership

| Behavior                                                         | Owner                                                    |
| ---------------------------------------------------------------- | -------------------------------------------------------- |
| Command usage and local administrator recovery                   | `internal/cli`                                           |
| HTTP sessions, CSRF, roles, grants, audit and scoped streams     | `internal/web`                                           |
| Document revisions and application coordination                  | `internal/ops`                                           |
| Derived health, topology and redaction                           | `internal/observe`                                       |
| YAML, variables, validation and atomic file editing              | `internal/projects`                                      |
| Release lifecycle, reservations, timeout, integrity and rollback | Focused `internal/pipeline` tests                        |
| Framing, handshake, role and payload contract                    | `internal/ufp`, with link composition in `internal/link` |
| Persistence, migrations, revisions and reconciliation            | `internal/storage/sqlite`                                |
| Agent job coordination and runtime policy                        | `internal/agent`                                         |
| Browser layout persistence and presentation rules                | `frontend/tests`                                         |
| Docker, registry and real replacement behavior                   | Explicit integration and live tests                      |

Add tests for protocol, persistence, security, deployment state, concurrency, untrusted input and confirmed regressions. Avoid trivial accessors, cosmetic text snapshots and duplicate ownership. Keep fixtures in the same package and split test files by behavior.

## Integration lane

```sh
docker info
make test-integration
```

The `integration` build tag composes application packages and uses temporary configuration and databases. Docker is explicit through `URUFLOW_DOCKER_TESTS=1`. Run this lane for server composition, persisted document changes, registry lifecycle and agent integration.

## Live Docker lane

```sh
make test-live
```

The `live` tag exercises a real Docker daemon, registry, pulls, readiness, exited-container logs, multi-service replacement and restoration. It may pull test images. Tests use isolated resource names and register cleanup before mutation. Run this lane when Docker, registry, agent runtime or deployment behavior changes.

Verify prerequisites and inspect skipped tests. A skipped infrastructure test is not evidence that the behavior passed.

## Race lane

```sh
make test-race
```

Run for sessions, streams, protocol, agent links, pipeline state and storage coordination. Keep race detection separate from the default suite so its cost and environment remain explicit.

## Browser preview

```sh
make test-preview
```

This builds the frontend and starts the integration-tagged browser fixture at `http://127.0.0.1:19002`. It uses temporary state, prints test-only credentials and sample projects, and requires Docker discovery. It does not dispatch real workloads or start its own registry. The preview is a visual review environment, not an end-to-end deployment test.

The fixture defaults to 30 minutes. Set `URUFLOW_BROWSER_QA_STOP` to a marker path before starting, then create that file to stop it. Set `URUFLOW_BROWSER_QA_LARGE=1` for the larger agent inventory.

For live reconnect review:

```sh
URUFLOW_BROWSER_QA_RESTART=/tmp/uruflow-browser-restart make test-preview
```

While signed in, create that marker file from another terminal. The fixture consumes it and restarts the web listener against the same database. The browser should reconnect without reloading or losing a valid session. Store-and-handler restart tests separately verify persistence after reopening SQLite.

## Browser scenarios

Review changed flows with a real browser at desktop and narrow widths. Check system, light and dark themes, keyboard navigation and reduced motion where affected.

For this workspace, regression review includes:

- Anonymous entry, first-admin setup, sign-in, password changes, scoped accounts and revocation.
- Project summary, service definitions in the graph inspector, release details and compatibility with existing Services-tab links.
- Fleet, runtime and delivery graph relationships, host expansion, search through collapsed hosts, focus and attachment controls.
- Quiet graph connections, dependency and planned-link disclosure, selection highlights, reduced motion, and unchanged positions and viewport on dense service graphs.
- Fleet server-to-agent and agent-to-project paths without redundant server-to-project edges; build links, stale placements and unassigned projects remain accurate.
- Two-axis trackpad panning without changing zoom or scrolling the page, pinch and modified-scroll zoom, mouse dragging and inspector scrolling.
- Non-overlapping card geometry, unavailable versus zero metrics, and live measurements without position or viewport changes.
- Saved graph layouts across reloads and card-size migration, consistent fit/reset framing and expanded graphs at desktop and narrow widths.
- Graph autosave after dragging and keyboard movement, persistence across reloads, retry after browser storage failure, and deployment and navigation remaining available.
- Configuration validation, unsaved-edit navigation and recovery after a revision conflict.
- One-time enrollment command protection and the permanent CA download.
- Live reconnect after listener restart and responsive actions with multiple browser tabs.
- Container disappearance while loaded logs remain visible, independent stdout/stderr cursor resume, batched replay after hidden tabs, dropped streams, terminal stream errors, completion and downloads.
- Fleet and project overviews with empty, degraded, stale and large observations; service readiness agrees with the infrastructure graph, and completed jobs remain visible after container cleanup.
- Graph pan and zoom retained while the inspector opens, accessible tabs, first-admin sign-in retry and cross-tab theme changes.
- Independent filtering, sorting and pagination when a page contains several tables.
- Page scrolling with the pointer over a table, log following across buffer trimming, and pausing to read older output without moving the surrounding page.
- Amber rollout progress despite temporary missing or stopped containers, with actual container failures and post-release degradation still visible.
- Primary destinations (Settings only for administrators), ownership of direct resource links and redirects from retired index pages, including `/system?tab=Settings`.
- Full-width YAML and Secrets & variables tabs, large-file scrolling, modified-tab indicators, and independent edits retained while switching.
- Simultaneous configuration and variable edits: saving or discarding one must preserve the other’s navigation guard.
- Combined release overview, first and subsequent release diffs, saved-definition access and stable tab selection during live progress.
- Scoped active and resolved alert history, event filters, and administrator-only audit access under System.
- Account-menu keyboard navigation, focus restoration, theme changes, password changes and sign-out confirmations.
- Read-only editors during pending saves and idle workspace rendering without a one-second update loop.
- Selective deployment and the Rebuild all images option; complete release records and live-release identification when unchanged containers keep running.
- Compressed asset negotiation, identity fallback, HEAD responses and matching decoded content.
- Empty fleets, unavailable metrics and state-dependent actions.
- Administrator-only Settings sections and their direct addresses: show-once webhook secrets and the previous secret's overlap, listener and certificate changes, delivery history, registry cleanup and password rotation, agent reporting, and recovery after a revision conflict.
- Administrator-only project deletion: refusal while a release runs or a runner is offline, the volume and image choices, and return to Projects with the project gone.

Use controlled responses to force browser error states when needed. Use real Docker and authenticated UFP agents for claims about deployment execution; browser fixtures verify presentation and interaction.

## Evidence for a change

Record the affected behavior, success and failure cases, commands, environment and relevant browser results in the change description. Keep generated assets, screenshots, credentials and temporary databases out of source control. Test counts belong to runner output, not a handwritten documentation total.

Related: [Contributing](../CONTRIBUTING.md), [Architecture](architecture.md), [Upgrading](upgrading.md).

Registry changes also verify bounded concurrent tag listing without manifest requests, partial results, cached reads during refresh, OCI index sizes and shared children. Retention tests cover pinned and observed images, distinct per-service history, expired restore rejection, and exclusion of releases and project edits during maintenance. The focused live registry test proves that garbage collection removes obsolete blobs while retained OCI images remain readable. Password rotation is refused while releases run, and the live rotation test proves the previous login stops working while the server's own client follows the new one.
