# Contributing

Thanks for looking at URUFLOW. This page covers what you need to build it, how to run the tests, and
the conventions the codebase follows.

## Building

Go 1.25 or later. No code generation, no build tags to remember.

```bash
go build -o uruflow ./cmd/uruflow-server
go build -o uruflow-agent ./cmd/uruflow-agent
```

`cmd/uruflow-server` builds the `uruflow` command; `cmd/uruflow-agent` builds `uruflow-agent`.

## Testing

There are two suites. The fast one runs anywhere:

```bash
go test ./...
go vet ./...
```

The second builds real images and starts real containers, and is gated behind an environment variable
so it never runs by accident:

```bash
URUFLOW_DOCKER_TESTS=1 go test ./...
```

The gate guards the tests in `internal/docker`, `internal/registry`, `internal/agent/runner`,
`internal/api`, `internal/tui` and `internal/tui/views` — everything that needs a real daemon, a real
registry, or a fully wired server. Run it before changing the release path, the Docker client, the
registry lifecycle or the composition root.

## Where a Change Belongs

[Architecture](docs/architecture.md#12-where-a-change-belongs) maps each kind of change to the package
that owns it. Two boundaries are worth knowing before you start:

- **`internal/ufp` imports nothing else in the repository.** It is the contract both the server and
  the agent implement. Adding an import there is a design change, not a refactor — raise it in an
  issue first.
- **`internal/tui` holds no logic.** It reads through the composition root in `internal/api` and never
  reaches into storage or the pipeline directly.

Changing the wire protocol means changing [docs/protocol.md](docs/protocol.md) in the same pull
request. Adding a method or topic is backward compatible; changing what an existing one means is not,
and needs a version bump.

## Code Style

The codebase is deliberately plain Go. Match what is already there:

- `gofmt` output, no exceptions
- no explanatory comments — only the MIT header block at the top of each file. Names carry the
  meaning; if a line needs a comment to be understood, rewrite the line
- named constants instead of inline literals, especially for anything that appears on the wire, in a
  path, or in a policy decision
- errors wrapped with context (`fmt.Errorf("read config: %w", err)`), never discarded silently
- new files start with the same MIT header block as every existing file

Prefer the smallest change that does the job. A new package or abstraction should be able to justify
itself.

## Documentation

Documentation lives in [docs/](docs/) and is expected to describe the code that exists. If a change
alters behaviour, a command, a configuration field, a default or a failure mode, update the affected
page in the same pull request. [README.md](README.md) links every document and is the front page —
keep deep detail in `docs/` and link to it rather than expanding the README.

## Pull Requests

- one concern per pull request
- `go build ./...`, `go test ./...` and `go vet ./...` clean before you open it
- say what you changed and why; if it touches the protocol, the schema or the file format, say what
  stays compatible and what does not
- commit subjects in this repository follow `Verb | short description`, for example
  `Fix | server releases url issue`

## Reporting Problems

For a bug, include the URUFLOW version (`uruflow version`), the platform, what you expected, what
happened, and the relevant lines from `<data_dir>/uruflow.log` or the agent's log file.

[Troubleshooting](docs/troubleshooting.md) lists the symptoms that already have known causes — worth a
look first.

Please do not open a public issue for a security problem. Report it privately to the maintainer
instead, with enough detail to reproduce it.
