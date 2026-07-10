#!/bin/sh
# Render deploy entrypoint. Render free has no persistent disk, so we reconstruct
# the SQLite DB from a private GitHub "data" repo on boot and push changes back
# on every mutation (the server's built-in JSONL+git export). If no backup env is
# configured, the server just runs with an ephemeral DB.
set -e

: "${TASKS_DB:=/data/tasks.db}"
BK=/data/backup
mkdir -p /data

if [ -n "$BACKUP_REPO_SSH" ] && [ -n "$BACKUP_SSH_KEY" ]; then
  echo "[entrypoint] configuring git backup -> $BACKUP_REPO_SSH"
  mkdir -p "$HOME/.ssh"
  printf '%s\n' "$BACKUP_SSH_KEY" > "$HOME/.ssh/id_ed25519"
  chmod 600 "$HOME/.ssh/id_ed25519"
  export GIT_SSH_COMMAND="ssh -i $HOME/.ssh/id_ed25519 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
  git config --global user.email "tasks@agenttasks.sh"
  git config --global user.name "tasks backup"
  git config --global init.defaultBranch main

  if [ ! -d "$BK/.git" ]; then
    if git clone "$BACKUP_REPO_SSH" "$BK" 2>/dev/null; then
      echo "[entrypoint] cloned backup repo"
    else
      echo "[entrypoint] empty/new backup repo — initializing"
      mkdir -p "$BK"; cd "$BK"; git init -q; git remote add origin "$BACKUP_REPO_SSH"
    fi
  fi
  cd "$BK"
  # Ensure a main branch with an upstream so the exporter's plain `git push` works.
  if ! git rev-parse --verify HEAD >/dev/null 2>&1; then
    git checkout -q -b main 2>/dev/null || git checkout -q main 2>/dev/null || true
    git commit -q --allow-empty -m "init backup" || true
    git push -q -u origin main 2>/dev/null || echo "[entrypoint] initial push deferred to first export"
  fi
  # Restore: fresh DB but a backup snapshot exists -> import it.
  if [ ! -f "$TASKS_DB" ] && [ -f "$BK/issues.jsonl" ]; then
    echo "[entrypoint] restoring DB from backup issues.jsonl"
    tasksd import --db "$TASKS_DB" "$BK/issues.jsonl" || echo "[entrypoint] restore failed (continuing empty)"
  fi
  export TASKS_EXPORT="$BK/issues.jsonl"
  export TASKS_GIT=true
  export TASKS_GIT_PUSH=true
else
  echo "[entrypoint] no backup configured — ephemeral DB (data will not persist across restarts)"
fi

echo "[entrypoint] starting tasksd"
cd /data
exec tasksd
