#!/usr/bin/env bash
# Mutation-tests one package and fails if the test efficacy (the share of
# injected bugs the tests catch) is below a minimum. gremlins' own
# --threshold-efficacy is ignored in v0.6.0, so the score is checked here.
#
# Usage: scripts/mutation.sh <package> [min-efficacy-percent]
#   scripts/mutation.sh ./internal/source 75
set -euo pipefail

pkg=${1:?package, e.g. ./internal/source}
min=${2:-75}
cd "$(dirname "$0")/.."

out=$(mktemp)
trap 'rm -f "$out"' EXIT
# The tests touch the filesystem, so allow each mutant's run more time.
# demo.go is demo mode's made-up data: not safety-critical, and its tests
# deliberately check shapes rather than exact numbers.
go tool -modfile=tools/go.mod gremlins unleash --workers 2 --timeout-coefficient 10 \
  --exclude-files 'demo\.go$' "$pkg" | tee "$out"

efficacy=$(sed -n 's/.*Test efficacy: \([0-9.]*\)%.*/\1/p' "$out" | tail -1)
if [ -z "$efficacy" ]; then
  echo "::error::no test efficacy reported for $pkg"
  exit 1
fi
if awk -v e="$efficacy" -v m="$min" 'BEGIN { exit !(e < m) }'; then
  echo "::error::test efficacy for $pkg is ${efficacy}%, below ${min}%"
  exit 1
fi
echo "test efficacy for $pkg: ${efficacy}% (minimum ${min}%)"
