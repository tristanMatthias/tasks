package main

// primeText is printed by `tasks prime` (and `bd prime` via the symlink). The
// SessionStart / PreCompact hooks call it to load workflow context.
const primeText = `# Tasks — workflow context

This project tracks work with the self-hosted "tasks" server (a Dolt-free
replacement for beads). All state lives in one SQLite database served by tasksd
and browsable in the web UI. Use the tasks CLI (or MCP tools on Claude Code web)
for ALL task tracking — do NOT use TodoWrite or markdown TODO lists.

## Core commands

  tasks ready              # Find claimable work (open, unblocked)
  tasks show <id>          # View task details
  tasks update <id> --claim  # Atomically claim work (assignee=you, in_progress)
  tasks close <id> -r "…"  # Complete work with a reason
  tasks create "<title>" -p 1 -t task   # File new work
  tasks dep add <blocked> <blocker>     # Record a blocks dependency
  tasks comment <id> "<text>"           # Add a comment

## Rules

- Claim before you start: tasks update <id> --claim
- Close only when 100% complete — never defer; leave it open instead.
- File follow-up work as new tasks rather than dropping it.
- Sync is automatic: the server is the single source of truth. There is no
  Dolt to push/pull; the UI reload button is a no-op.

Run "tasks --help" for the full command list.
`
