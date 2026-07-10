#!/usr/bin/env bash
# Build, (first-run) import, serve, and expose the tasks server over Tailscale
# Funnel so Claude Code web can reach the MCP endpoint.
#
#   TASKS_TOKEN=... ./scripts/run.sh
#
# Env:
#   TASKS_TOKEN  bearer token (REQUIRED for a public funnel)
#   PORT         listen port (default 7842)
#   DB           sqlite path (default data/tasks.db)
#   IMPORT       issues.jsonl to import if the db is empty
#   EXPORT       issues.jsonl to mirror to on change (backup)
set -euo pipefail
cd "$(dirname "$0")/.."

PORT="${PORT:-7842}"
DB="${DB:-data/tasks.db}"
IMPORT="${IMPORT:-../forge-crafting-intepreters/.beads/issues.jsonl}"
EXPORT="${EXPORT:-../forge-crafting-intepreters/.beads/issues.jsonl}"

if [[ -z "${TASKS_TOKEN:-}" ]]; then
  echo "WARNING: TASKS_TOKEN is not set — the funnel would be UNAUTHENTICATED." >&2
  echo "Generate one:  export TASKS_TOKEN=\$(openssl rand -hex 32)" >&2
fi

go build -o bin/tasksd ./cmd/tasksd
go build -o bin/tasks  ./cmd/tasks
ln -sf tasks bin/bd

# Start the funnel in the background (public HTTPS -> localhost:$PORT).
if command -v tailscale >/dev/null 2>&1; then
  echo "Exposing port $PORT via Tailscale Funnel…"
  tailscale funnel --bg "$PORT" || echo "(funnel setup failed; serving locally only)"
  echo "Funnel URL: $(tailscale funnel status 2>/dev/null | grep -o 'https://[^ ]*' | head -1 || echo 'see: tailscale funnel status')"
  echo "Register the MCP endpoint in Claude Code web at:  <funnel-url>/mcp"
else
  echo "tailscale not found — serving on localhost only." >&2
fi

exec bin/tasksd --addr "127.0.0.1:$PORT" --db "$DB" --import "$IMPORT" --export "$EXPORT"
