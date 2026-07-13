#!/usr/bin/env bash
# E2E gate: a commit that changes UI feature code (web/ui/src/**) MUST also add
# or update an end-to-end spec (web/ui/e2e/**/*.spec.ts). This keeps every UI
# feature/behavior change covered by a test.
#
# Legitimate non-feature UI commits (refactors, style-only, comment/typo fixes,
# dependency bumps) can bypass with:
#
#     ALLOW_NO_E2E=1 git commit ...
#
# Backend (Go) changes are covered by the separate coverage gate, so this only
# looks at the UI source tree.
set -euo pipefail

if [[ "${ALLOW_NO_E2E:-}" == "1" ]]; then
  echo "e2e gate: bypassed (ALLOW_NO_E2E=1)"
  exit 0
fi

staged="$(git diff --cached --name-only --diff-filter=ACMR)"

# No UI source touched? Nothing to enforce.
if ! grep -qE '^web/ui/src/' <<<"$staged"; then
  exit 0
fi

# UI source touched — require a staged e2e spec change.
if grep -qE '^web/ui/e2e/.*\.spec\.ts$' <<<"$staged"; then
  echo "e2e gate: OK (UI change ships with an e2e spec)"
  exit 0
fi

cat >&2 <<'MSG'

FAIL: e2e gate.
UI code under web/ui/src/ changed, but no end-to-end spec was added or updated.

Every feature/behavior change to the UI must ship an E2E test:
  • add or extend a spec in web/ui/src/../e2e/*.spec.ts (compose the board bricks)
  • verify it locally with:  cd web/ui && npm run test:e2e

If this commit genuinely introduces no feature/behavior (refactor, style, docs,
comments, dep bump), bypass the gate:
  ALLOW_NO_E2E=1 git commit ...

MSG
exit 1
