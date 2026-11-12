#!/usr/bin/env bash
# Generates shell completions shipped in release archives and packages.
set -euo pipefail
rm -rf completions
mkdir -p completions
for sh in bash zsh fish powershell; do
  ext=$sh
  [ "$sh" = powershell ] && ext=ps1
  go run ./cmd/diskdiet completion "$sh" > "completions/diskdiet.$ext"
done
