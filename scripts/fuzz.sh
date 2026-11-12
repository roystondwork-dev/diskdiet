#!/usr/bin/env bash
# Runs every fuzz target in the module for FUZZTIME (default 30s) each.
# Usage: scripts/fuzz.sh [fuzztime]
set -euo pipefail

fuzztime=${1:-30s}
cd "$(dirname "$0")/.."
for pkg in $(go list ./...); do
  for target in $(go test -list '^Fuzz' "$pkg" | grep '^Fuzz' || true); do
    echo "==> $pkg $target"
    go test -run='^$' -fuzz="^${target}\$" -fuzztime="$fuzztime" "$pkg"
  done
done
