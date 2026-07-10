# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Self-hosted, Dolt-free task server (`tasksd`) — SQLite single source of truth,
  serving the browser UI, a REST API, and an MCP endpoint over one listener.
- `tasks` CLI (drop-in for the used `bd` command subset; ships a `bd` symlink).
- Single command registry (`internal/api`): HTTP, MCP, and CLI surfaces are all
  generated from one declarative definition with shared validation.
- Lossless import of an existing beads `issues.jsonl`; optional JSONL + git backup.
- Bearer-token / cookie auth, with a fail-safe that refuses to run unauthenticated
  on a public bind.
- Production hardening: env-var config (`TASKS_*`), structured logging (`slog`,
  text/json), panic recovery, request IDs, access logs, security headers, CORS,
  request body limits, per-IP rate limiting.
- Operational endpoints: `/healthz`, `/readyz`, `/version`, `/metrics` (Prometheus).
- Packaging: static multi-arch Docker image (scratch, non-root), Compose file,
  systemd unit, `.env.example`.
- CI (test + vet + coverage gate + cross-build + docker) and a tagged release
  workflow (binaries + GHCR image).

[Unreleased]: https://github.com/tristanMatthias/tasks
