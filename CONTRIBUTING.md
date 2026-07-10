# Contributing

Thanks for helping improve `tasks`.

## Development setup

```bash
git clone <repo> && cd tasks
make hooks        # enable the pre-commit coverage gate
make build test   # build binaries + run the suite
```

Requirements: Go 1.25+. No CGO — the SQLite driver is pure Go, so builds are
static and cross-compile trivially (`CGO_ENABLED=0`).

## Adding or changing a command

Every command is defined **once** in `internal/api/ops.go` (an input struct +
a handler over `core`). The HTTP route, MCP tool, and CLI subcommand are all
generated from that registry by reflection — so add the op there and it appears
on all three surfaces automatically. Add validation in `internal/api/validate.go`
if the field needs constraints.

## Tests & coverage

- `make test` — run everything.
- `make cover` — total coverage across packages.
- `make cover-check` — the gate the pre-commit hook runs.

Coverage is measured with `-coverpkg=./...` so integration tests (httpapi, MCP)
count toward the packages they exercise. The gate threshold lives in
`.coverage-min` (or the `COVERAGE_MIN` env var). Keep new code covered.

## Style

- Match the surrounding code; keep comments focused on the *why*.
- Run `gofmt` (CI enforces it) and `go vet`.
- Prefer standard-library solutions; new dependencies should be well-justified.

## Commit / PR

- Small, focused PRs with a clear description.
- CI must be green (fmt, vet, coverage gate, cross-build, docker build).
- Note any user-facing changes in `CHANGELOG.md`.
