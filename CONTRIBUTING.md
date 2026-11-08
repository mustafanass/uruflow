# Contributing

Changes should preserve URUFLOW's ownership boundaries, keep its operator workflow clear, and include evidence for the behavior they change.

## Build and check

Use the Go version in [go.mod](go.mod), Node.js 24, npm and a C compiler for SQLite. Install Docker for the integration and live lanes.

```sh
make frontend-deps
make fmt
make check
make build
```

Run these as one validation pass after implementation. If a check fails, fix the cause and repeat the affected pass. CI runs `make check` and `make build` on pushes and pull requests.

[Testing](docs/testing.md) defines test ownership, the deterministic suite, Docker lanes, race checks and browser review. Keep infrastructure-dependent tests explicit.

## Building a release

Install [Zig](https://ziglang.org/download/) alongside the normal build tools. Release builds use
`zig cc` with musl to link SQLite into static Linux binaries. Zig 0.15.2 is the validated compiler.
The Go and frontend versions must match in `internal/version/version.go` and `frontend/package.json`.
Update the frontend lockfile and README version examples with them.

From the repository root:

```sh
make check
make release
```

`make release` builds the production frontend once, embeds it in the server, and writes these files to `dist/v3.0.5/`:

| Asset                                    | Contents                                                  |
| :--------------------------------------- | :-------------------------------------------------------- |
| `uruflow-3.0.5-linux-amd64.tar.gz`       | Server for Linux x86-64, its service unit and MIT license |
| `uruflow-3.0.5-linux-arm64.tar.gz`       | Server for Linux ARM64, its service unit and MIT license  |
| `uruflow-agent-3.0.5-linux-amd64.tar.gz` | Agent for Linux x86-64, its service unit and MIT license  |
| `uruflow-agent-3.0.5-linux-arm64.tar.gz` | Agent for Linux ARM64, its service unit and MIT license   |
| `SHA256SUMS.txt`                         | SHA-256 checksums for the four archives                   |

Verify each architecture on Linux: run both `version` commands, inspect the embedded workspace,
and exercise SQLite and the affected deployment paths. The [testing lanes](docs/testing.md)
define the additional checks for a change. Build from the same committed checkout that the release tag names.

After committing and pushing the release changes, create and push the tag, then upload the files:

```sh
git tag -a v3.0.5 -m "URUFLOW 3.0.5"
git push origin v3.0.5
gh release create v3.0.5 dist/v3.0.5/* --verify-tag --title "URUFLOW 3.0.5" --generate-notes
```

Alternatively, create release `v3.0.5` in GitHub and attach all five files from `dist/v3.0.5/`.
Publishing the release makes the README's pinned download URLs available.
[`--verify-tag`](https://cli.github.com/manual/gh_release_create) prevents GitHub CLI from creating
a tag against an unintended commit. Building locally does not upload files or create a release.

## Frontend development

Run the backend with a development configuration on its loopback operator address. For frontend hot reload, start Vite in another terminal:

```sh
URUFLOW_API_URL=http://127.0.0.1:9002 npm --prefix frontend run dev
```

Open the address Vite prints. Its `/api` proxy uses the running backend, including authentication and live streams. Use a development installation with matching origin settings. Production uses `make build` and serves the embedded assets from the Go server.

## Where a change belongs

| Change                                         | Owner                                                      |
| ---------------------------------------------- | ---------------------------------------------------------- |
| Browser routes, providers and shell            | `frontend/src/app`                                         |
| Project, agent, container or release workflow  | Its domain under `frontend/src/features`                   |
| Reusable controls, API access, hooks and types | `frontend/src/shared`                                      |
| Colors, motion and responsive behavior         | `frontend/src/styles`                                      |
| HTTP access, sessions and browser streams      | `internal/web`                                             |
| Application operations and document saves      | `internal/ops`                                             |
| Observations and derived topology              | `internal/observe`                                         |
| Deployment coordination                        | `internal/pipeline`                                        |
| Agent builds, runtime and metrics              | `internal/agent`                                           |
| Server-agent contract                          | `internal/ufp`                                             |
| Configuration files and persistence            | `internal/projects`, `internal/config`, `internal/storage` |

The [architecture](docs/architecture.md) explains the boundaries and the rules each change must preserve.

## Code style

Keep files focused on one responsibility and match the surrounding code. Use plain control flow and descriptive names before adding abstractions.

Go uses `gofmt`, uppercase initialisms such as `ID` and `HTTP`, `any`, and errors wrapped with useful context. Preserve the original MIT copyright and license header in every Go file, including tests, and keep compiler directives intact. Avoid explanatory source comments unless requested.

Frontend code uses the existing `app`, `features` and `shared` structure, TypeScript, single quotes, no semicolons, two-space indentation and Prettier. Keep source code free of explanatory comments. Use the shared color tokens and controls. UI text should describe the next action or the observed state in a few words.

## Changes and evidence

Add focused tests for security, persistence, protocol behavior, deployment state, concurrency, untrusted input and confirmed regressions. Keep tests with their owner and avoid duplicating another package's assertions. Visual changes need browser review at desktop and narrow widths, including keyboard use and the supported themes.

Keep project authorization on the server, validate resolved documents before writes, and preserve release snapshots and persisted history. Update [UFP](docs/protocol.md) in the same change when a method, topic or payload changes.

A pull request should explain the problem, resulting behavior, commands run, and any schema or protocol rollout steps. Use the repository's commit-subject form, for example `Fix | preserve sessions across server restarts`.

## Documentation

The README is the product entry point. Detailed guidance belongs in `docs/`. Each topic has one home; link to it elsewhere. Describe the code that exists, use exact command and UI names, and update related examples and links together. Keep release notes focused on the current upgrade path.

Report bugs with the server and agent versions, platform, expected result, actual result and relevant logs. Report security issues privately to the maintainer.
