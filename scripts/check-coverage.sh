#!/usr/bin/env bash
# Coverage gate. Runs the whole suite with integrated coverage and fails below
# the threshold.
#
#   ./scripts/check-coverage.sh
#
# Threshold resolution (first match wins):
#   COVERAGE_MIN env var  ->  .coverage-min file  ->  100
#
# We use Go 1.20+ integration coverage (GOCOVERDIR + `go tool covdata`) rather
# than a single -coverprofile. `go test -coverpkg=./... -coverprofile ./...`
# does NOT correctly merge `main`-package coverage across the per-package test
# binaries (a directly-tested cmd/ function reads as ~50% in the merged profile
# yet 100% in isolation). covdata merges the raw counter dirs from every test
# binary correctly, so main packages are counted properly.
set -euo pipefail
cd "$(dirname "$0")/.."

MIN="${COVERAGE_MIN:-$( [[ -f .coverage-min ]] && cat .coverage-min || echo 100 )}"

COVDIR="$(mktemp -d)"
trap 'rm -rf "$COVDIR"' EXIT

echo "running tests with coverage (threshold ${MIN}%)…"
go test -coverpkg=./... -covermode=atomic ./... -args -test.gocoverdir="$COVDIR" >/dev/null

# Merge all binaries' counters into a single legacy profile + compute the total.
go tool covdata textfmt -i="$COVDIR" -o=coverage.out
TOTAL="$(go tool cover -func=coverage.out | awk '/^total:/ {gsub(/%/,"",$3); print $3}')"
echo "total coverage: ${TOTAL}%"

if awk -v t="$TOTAL" -v m="$MIN" 'BEGIN { exit !(t < m) }'; then
  echo ""
  echo "FAIL: coverage ${TOTAL}% is below the required ${MIN}%."
  echo "Functions below 100%:"
  go tool cover -func=coverage.out | awk '$3 != "100.0%" && $1 != "total:"' | sed 's/^/  /'
  echo ""
  echo "Raise coverage, or set COVERAGE_MIN / edit .coverage-min to change the gate."
  exit 1
fi

echo "OK: coverage ${TOTAL}% meets the ${MIN}% threshold."
