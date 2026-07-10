# tasks

A self-hosted, **Dolt-free** replacement for [beads](https://github.com/gastownhall/beads) —
one Go binary that is the **single source of truth** for multi-agent task tracking, reachable by
Claude Code web over a **Tailscale Funnel**. Same browser UI, a `bd`-compatible CLI, an MCP server,
and a REST API — all generated from one command registry.

## Why

beads stores issues in an embedded Dolt database and syncs them through git refs. That layer breaks
constantly and is fragile under concurrent multi-agent use. `tasks` replaces it with:

- **SQLite as the single source of truth** (pure-Go `modernc.org/sqlite`, WAL) — transaction-atomic
  claims mean two agents can never claim the same task.
- **One server, many surfaces**: the browser UI, REST, MCP (for Claude Code web), and the CLI all
  talk to the same process. No Dolt, no ref sync, no "which copy is right".
- **Tighter control than beads**: every write is validated (status/type/priority enums, required
  fields) uniformly across all surfaces.
- **beads-compatible data**: imports an existing `.beads/issues.jsonl` losslessly and keeps exporting
  the same format (optionally git-committed) as an off-machine backup.

## Architecture

```
                    ┌──────────────── tasksd (one process) ────────────────┐
 Claude Code web ──▶│  /mcp   (MCP)     ┐                                    │
 tasks CLI / bd  ──▶│  /api/v1/* (REST) ┼─▶ internal/api registry ─▶ core ─▶ │ SQLite (source of truth)
 browser UI      ──▶│  /api/issues (UI) ┘         (single source)           │      │
                    │  web/static (embedded UI)                             │      ▼
                    └───────────────────────────────────────────────────────┘   issues.jsonl (+git backup)
```

Every command (`ready`, `show`, `list`, `create`, `update`, `claim`, `close`, `dep`, `comment`) is
declared **once** in `internal/api/ops.go`. The HTTP routes, MCP tools, and CLI subcommands are all
generated from that registry by reflection, so a field added there appears on every surface.

## Quick start

```bash
# 1. Build (produces bin/tasksd, bin/tasks, and a bd -> tasks symlink)
make build

# 2. Import your existing beads data (SQLite becomes the source of truth)
make import                       # imports ../forge-crafting-intepreters/.beads/issues.jsonl

# 3. Serve + expose publicly for Claude Code web
export TASKS_TOKEN=$(openssl rand -hex 32)
./scripts/run.sh                  # builds, serves on :7842, runs `tailscale funnel`
```

Open the UI at `http://127.0.0.1:7842/`. To authenticate the browser once behind the funnel, visit
`https://<funnel-host>/auth?token=<TASKS_TOKEN>` (sets an httpOnly cookie).

### Or with Docker

```bash
cp .env.example .env          # set TASKS_TOKEN=$(openssl rand -hex 32)
docker compose up -d          # static, non-root, read-only rootfs, /data volume
```

## Deploy

- **Docker** — a static multi-arch image (scratch base, non-root uid, read-only
  rootfs) is built from the `Dockerfile`; `docker-compose.yml` wires a `/data`
  volume, healthcheck, and env. Prebuilt images publish to `ghcr.io/<owner>/tasks`.
- **systemd** — `deploy/tasksd.service` runs it as a locked-down system service
  (`ProtectSystem=strict`, `NoNewPrivileges`, `MemoryDenyWriteExecute`, …).
- **Tailscale** — `make funnel` exposes the port publicly over HTTPS; `make
  serve-tailnet` keeps it tailnet-only.

## Configuration

Everything is configurable via `TASKS_*` env vars or the matching `--flag`
(flags override env override defaults). See **`.env.example`** for the full,
documented list. Highlights:

| Env | Flag | Default | Purpose |
|---|---|---|---|
| `TASKS_ADDR` | `--addr` | `127.0.0.1:7842` | listen address |
| `TASKS_DB` | `--db` | `data/tasks.db` | SQLite path |
| `TASKS_TOKEN` / `TASKS_TOKEN_FILE` | `--token` / `--token-file` | — | bearer token |
| `TASKS_LOG_FORMAT` | `--log-format` | `text` | `text` or `json` |
| `TASKS_RATE_LIMIT` | `--rate-limit` | `20` | per-IP req/s (0 disables) |
| `TASKS_MAX_BODY_BYTES` | `--max-body-bytes` | `1048576` | request body cap |
| `TASKS_EXPORT` | `--export` | — | mirror store to JSONL on change |
| `TASKS_CORS_ORIGINS` | `--cors-origins` | — | allowed CORS origins |

`tasksd version` prints build metadata; `tasksd import`/`export` are one-shot
data commands.

## Observability & hardening

- **Structured logging** (`slog`), `text` or `json`, with per-request access
  logs carrying a request id (`X-Request-Id`, honored inbound).
- **Ops endpoints (unauthenticated):** `GET /healthz` (liveness), `GET /readyz`
  (DB ping), `GET /version`, `GET /metrics` (Prometheus text).
- **Middleware:** panic recovery, security headers (CSP, `X-Frame-Options`, …),
  per-IP token-bucket rate limiting, request body-size limits, optional CORS.
- **Graceful shutdown** on SIGINT/SIGTERM with a configurable drain timeout.
- See **`SECURITY.md`** for the self-hosting hardening checklist.

## Using the CLI

> **The CLI is a client — `tasksd` must be running.** Start it first (`make serve`,
> `./bin/tasksd`, Docker, or the systemd/launchd service in `deploy/`). If the CLI
> can't reach a tasks server (or hits a *different* server on the port) it now says
> so explicitly instead of failing cryptically. On macOS, `deploy/com.tasks.tasksd.plist`
> keeps it running via launchd.

The CLI talks to the server over HTTP. Point it at the server and authenticate:

```bash
export TASKS_URL=http://127.0.0.1:7842      # or your funnel URL
export TASKS_TOKEN=<token>

tasks ready                 # claimable work
tasks show <id>             # details
tasks create "Fix X" -p 1 -t bug
tasks update <id> --claim   # atomically claim
tasks close <id> -r "done"
tasks dep <blocked> <blocker>
tasks --json ready          # raw JSON (matches the REST shape)
```

Symlinking `tasks` as `bd` keeps existing beads workflows (`bd ready`, `bd prime`, hooks) working
unchanged — see `make install`.

**Short ids:** any command that takes a task id accepts the short form (without the project prefix) —
`tasks show w7t0` resolves to `tasks-w7t0`, `tasks update w7t0.1 --claim`, etc. A literal full-id
match always wins; the prefix is only prepended when the bare id doesn't exist.

## MCP for Claude Code web

The server exposes streamable-HTTP MCP at `/mcp`, guarded by the same bearer token. Register it in
Claude Code web with your funnel URL (`https://<host>/mcp`) and an `Authorization: Bearer <token>`
header. Tools: `ready`, `show`, `list`, `create`, `update`, `claim`, `close`, `dep`, `comment`.

## Auth

The server binds `127.0.0.1` and is fronted by `tailscale funnel` (public HTTPS). Because the funnel
is world-reachable, set `TASKS_TOKEN`; requests then require `Authorization: Bearer <token>` (CLI,
MCP, curl) or the `tasks_token` cookie (browser, via `/auth?token=`). With no token the server runs
open (dev only) and logs a warning. For tailnet-only access instead of public, use `make serve-tailnet`.

## Backup

Pass `--export <path>` (and optionally `--git` / `--git-push`) so the server mirrors the store to a
beads-format `issues.jsonl` after each change (debounced), giving off-machine recovery and keeping
any file-based tooling working. `tasksd export --db … --out …` does a one-shot export.

## Development

```bash
make test          # run all tests
make cover         # total coverage across packages
make cover-html    # coverage.html report
make vet
```

## Command reference

| CLI / MCP tool | REST | Description |
|---|---|---|
| `ready`   | `GET /api/v1/ready`                 | Claimable work (open, unblocked), priority-ordered |
| `list`    | `GET /api/v1/tasks`                 | Filter by status/type/assignee/label |
| `search`  | `GET /api/v1/search`                | Fuzzy text search (fzf/fuse.js-style ranking), best matches first |
| `tree`    | `GET /api/v1/tree`                  | A task's subtree (id optional — omit to render the whole forest) |
| `show`    | `GET /api/v1/tasks/{id}`            | Full task details |
| `create`  | `POST /api/v1/tasks`                | Create a task (id minted) |
| `update`  | `PATCH /api/v1/tasks/{id}`          | Update fields / `--claim` |
| `claim`   | `POST /api/v1/tasks/{id}/claim`     | Atomically claim |
| `close`   | `POST /api/v1/tasks/{id}/close`     | Close with reason |
| `dep`     | `POST /api/v1/deps`                 | Add a dependency |
| `comment` | `POST /api/v1/tasks/{id}/comments`  | Add a comment |

UI-compatibility endpoints (`GET /api/issues`, `GET /api/meta`, `POST /api/pull`) mirror the old
Python `beads_ui` server so the existing frontend works unchanged; `/api/pull` is a no-op (there is
no Dolt to pull).

## Notes on `ready` semantics

`ready` returns `open` tasks with **no unclosed `blocks` blocker**, priority- then id-ordered.
Parent-child links are treated as containment, *not* as blockers — beads inconsistently hides a child
when its parent is open, which can bury workable leaf tasks; `tasks` does not replicate that quirk.
The raw dependency data is imported untouched; only this readiness view differs.
