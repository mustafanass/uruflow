# Testing

Tests live with the package that owns the behavior. Cross-package tests cover only the integration
between those packages.

## Default suite

```bash
make check
```

The default suite runs formatting verification, `go vet ./...`, and `go test ./...`. It cannot depend
on Docker, the external network, root privileges, fixed ports, or operator input. Local temporary
files, SQLite databases, and test-assigned listeners are allowed.

## Test ownership

| Behavior | Owning test package |
| :--- | :--- |
| Command syntax, validation, usage and interaction metadata | `internal/grammar` |
| Completion stages and resource projection | `internal/workbench` completion tests |
| Inline YAML editing and recovery | `internal/workbench` editor tests |
| Workspace rendering and terminal width | `internal/workbench` view tests |
| Release lifecycle, concurrency, integrity, services and secrets | Focused `internal/pipeline` test files |
| UFP framing, authentication and payload compatibility | `internal/ufp` |
| Persistence, migrations and reconciliation | `internal/storage/sqlite` |
| Docker-backed runtime behavior | `live` tests |

Add tests for protocol, persistence, security, deployment state, concurrency, untrusted input, and
confirmed regressions. Avoid tests for accessors, cosmetic text, or behavior already owned elsewhere.

## Integration suite

```bash
make test-integration
```

Integration tests use the `integration` build tag and may compose several packages or start managed
infrastructure. They require Docker.

## Live Docker suite

```bash
make test-live
```

Live tests use the `live` build tag and exercise Docker, the registry, image pulls, readiness,
replacement, and rollback. They use isolated resource names and register cleanup before mutation.

## Race and preview suites

```bash
make test-race
make test-preview
```

Run the race suite for concurrency, streaming, protocol, link, pipeline, control, and workbench
changes. Preview tests print representative terminal output for manual inspection.

## File structure

Keep shared fixtures in a named helper file in the same package. Split files by behavior, using names
such as `concurrency_test.go`, `integrity_test.go`, `editor_test.go`, or `view_test.go`.
