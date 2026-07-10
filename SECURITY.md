# Security Policy

## Reporting a vulnerability

Please report security issues privately via GitHub's **"Report a vulnerability"**
(Security → Advisories) rather than opening a public issue. We aim to acknowledge
reports within 72 hours.

## Hardening checklist for self-hosters

`tasks` is built to be exposed to the public internet (e.g. over a Tailscale
Funnel so Claude Code web can reach it). Before exposing it:

- **Set `TASKS_TOKEN`.** The server refuses to start on a non-loopback bind
  without a token (override only with `--allow-no-auth`). Prefer `TASKS_TOKEN_FILE`
  or a container/orchestrator secret over passing `--token` on the command line
  (which is visible in the process list).
- **Rotate the token** by changing `TASKS_TOKEN` and restarting; browser sessions
  re-authenticate via `/auth?token=`.
- **Keep the rate limiter on** (`TASKS_RATE_LIMIT`, default 20 req/s per IP).
- **Only set `TASKS_BEHIND_PROXY=true`** when the server truly sits behind a proxy
  you control that sets `X-Forwarded-For`; otherwise clients can spoof their IP.
- **Restrict CORS**: leave `TASKS_CORS_ORIGINS` empty (same-origin) unless you
  serve the API to a known web origin.
- Run as a non-root user (the Docker image and systemd unit already do), on a
  read-only filesystem with a writable `/data` volume for the SQLite database.
- Back up `issues.jsonl` (`TASKS_EXPORT`) off-host; optionally `TASKS_GIT_PUSH`.

## What auth protects

The bearer token guards the UI, the REST API, and the MCP endpoint. The
operational endpoints `/healthz`, `/readyz`, `/version`, and `/metrics` are
intentionally **unauthenticated** for probing/monitoring — do not expose
`/metrics` publicly if you consider request counts sensitive (front it or set
`TASKS_METRICS=false`).
