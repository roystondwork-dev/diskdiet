#!/usr/bin/env bash
# Integration tests: real package managers fill real caches, diskdiet cleans
# them, and installs still work afterwards. Nothing outside a cache may change.
#
# Usage: scripts/integration.sh <path to diskdiet binary>
# Runs in CI on Linux, macOS and Windows (Git Bash). Everything runs under a
# temporary fake home folder, so neither the package managers nor diskdiet's
# own default lookups can reach the machine's real caches.
set -euo pipefail

# diskdiet cleans for real here. Make running it on a developer's machine a
# deliberate choice, even though the fake home keeps real caches out of reach.
if [ -z "${CI:-}" ] && [ "${DISKDIET_INTEGRATION_LOCAL:-}" != "1" ]; then
  echo "integration.sh runs real cleans; set DISKDIET_INTEGRATION_LOCAL=1 to run it outside CI" >&2
  exit 2
fi

dd=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"

# A fake home: every tool (and diskdiet's default cache locations) resolve here.
real_home=$HOME
export HOME="$work/home"
export USERPROFILE="$HOME"
export XDG_CACHE_HOME="$HOME/.cache" XDG_DATA_HOME="$HOME/.local/share" XDG_CONFIG_HOME="$HOME/.config"
export LOCALAPPDATA="$HOME/AppData/Local" APPDATA="$HOME/AppData/Roaming"
export GOTOOLCHAIN=local GOPATH="$HOME/go"
mkdir -p "$HOME" "$LOCALAPPDATA" "$APPDATA" "$XDG_CACHE_HOME" "$XDG_DATA_HOME" "$XDG_CONFIG_HOME"
case "$HOME" in "$real_home"*) echo "fake home is inside the real one; refusing" >&2; exit 2 ;; esac

pass() { printf '  ✓ %s\n' "$*"; }
fail() { printf '  ✗ %s\n' "$*" >&2; exit 1; }
step() { printf '\n== %s\n' "$*"; }
# A source's bytes and prunable bytes from `diskdiet scan --json`.
# shellcheck disable=SC2086 # $3 holds extra flags on purpose
field() { "$dd" scan --json --only "$1" ${3:-} 2>/dev/null | jq -r ".sources[0].$2 // 0"; }
py=$(command -v python3 || command -v python)
# Makes files and folders look 90 days old (portable, unlike touch -d).
make_old() { "$py" -c 'import os,sys,time; t=time.time()-90*86400; [os.utime(p,(t,t)) for p in sys.argv[1:]]' "$@"; }
# Global installs only on CI runners, never on a developer's machine.
install_global() { if [ -n "${CI:-}" ]; then npm install -g --silent "$1" >/dev/null 2>&1 || true; fi; }

# Every cache lives under $work, through each tool's own setting.
export npm_config_cache="$work/npm-cache"
export YARN_CACHE_FOLDER="$work/yarn-cache"
export PIP_CACHE_DIR="$work/pip-cache"
export GOMODCACHE="$work/gomod"
export GOCACHE="$work/gocache"
export GOFLAGS=-modcacherw

# A file outside every cache, linked from inside one: it must survive.
sentinel="$work/outside/keep-me.txt"
mkdir -p "$work/outside" && echo "precious" > "$sentinel"

project() { # name → a node project with one dependency
  mkdir -p "$work/$1" && cd "$work/$1"
  echo "{\"name\":\"$1\",\"version\":\"1.0.0\",\"private\":true}" > package.json
}

step "npm: recent entries stay, --all clears, installs still work"
project npm-app
npm install --silent --no-audit --no-fund is-number@7.0.0 >/dev/null
ln -s "$sentinel" "$npm_config_cache/_cacache/escape-link" 2>/dev/null || true
[ "$(field npm bytes)" -gt 0 ] || fail "npm cache not found"
[ "$(field npm prunableBytes '--older-than 1h')" = "0" ] || fail "fresh npm entries counted as old"
pass "fresh entries are kept with --older-than 1h"
"$dd" clean --only npm --all --yes >/dev/null
npm cache verify >/dev/null || fail "npm cache verify failed after clean"
rm -rf node_modules package-lock.json
npm install --silent --no-audit --no-fund is-number@7.0.0 >/dev/null || fail "npm install failed after clean"
pass "cache verifies and npm install works after --all"

step "node_modules: old projects go, recent ones stay"
mkdir -p "$work/projects"
for name in old-app new-app; do
  mkdir -p "$work/projects/$name/node_modules/dep"
  echo '{}' > "$work/projects/$name/package.json"
  echo 'module.exports=1' > "$work/projects/$name/node_modules/dep/index.js"
done
make_old "$work/projects/old-app/package.json" "$work/projects/old-app"
"$dd" clean --only node_modules --projects "$work/projects" --older-than 30d --yes >/dev/null
[ ! -e "$work/projects/old-app/node_modules" ] || fail "old project's node_modules kept"
[ -e "$work/projects/new-app/node_modules/dep/index.js" ] || fail "recent project's node_modules removed"
[ -e "$work/projects/old-app/package.json" ] || fail "old project's own files touched"
pass "only the untouched project's node_modules removed"
mkdir -p "$work/projects/kept/node_modules"
echo '{}' > "$work/projects/kept/package.json"
make_old "$work/projects/kept/package.json" "$work/projects/kept"
"$dd" clean --only node_modules --projects "$work/projects" --older-than 30d --keep "$work/projects/kept/node_modules" --yes >/dev/null
[ -e "$work/projects/kept/node_modules" ] || fail "--keep folder removed"
pass "--keep leaves a folder alone"

step "Yarn 1"
install_global yarn@1
if command -v yarn >/dev/null 2>&1; then
  project yarn-app
  yarn add --silent is-number@7.0.0 >/dev/null 2>&1
  [ "$(field yarn bytes)" -gt 0 ] || fail "Yarn cache not found"
  "$dd" clean --only yarn --all --yes >/dev/null
  rm -rf node_modules yarn.lock
  yarn add --silent is-number@7.0.0 >/dev/null 2>&1 || fail "yarn add failed after clean"
  pass "Yarn cache cleared and yarn add works"
else
  pass "Yarn unavailable here; skipped"
fi

step "pnpm: only packages no project uses are pruned"
install_global pnpm@10
if command -v pnpm >/dev/null 2>&1; then
  project pnpm-app
  store="$work/pnpm-store"
  # Git Bash paths (/tmp/...) mean something else to pnpm on Windows.
  if command -v cygpath >/dev/null 2>&1; then store=$(cygpath -m "$store"); fi
  echo "store-dir=$store" > .npmrc
  pnpm add --silent is-number@7.0.0 >/dev/null 2>&1
  before=$(field pnpm bytes)
  [ "$before" -gt 0 ] || fail "diskdiet didn't find the pnpm store ($store)"
  "$dd" clean --only pnpm --all --yes >/dev/null
  [ -e node_modules/is-number/package.json ] || fail "pnpm prune broke a project in use"
  pass "a project in use survives pnpm store prune (store: $before bytes)"
else
  pass "pnpm unavailable here; skipped"
fi

step "pip"
"$py" -m pip download --quiet --dest "$work/pip-dl" six==1.16.0 >/dev/null 2>&1
[ "$(field pip bytes)" -gt 0 ] || fail "pip cache not found"
"$dd" clean --only pip --all --yes >/dev/null
[ "$(field pip bytes)" = "0" ] || [ "$(field pip items)" = "0" ] || fail "pip cache not cleared"
"$py" -m pip download --quiet --dest "$work/pip-dl2" six==1.16.0 >/dev/null 2>&1 || fail "pip download failed after clean"
pass "pip cache cleared and pip works"

step "Go"
mkdir -p "$work/goapp" && cd "$work/goapp"
cat > go.mod <<'GOMOD'
module example.com/goapp
go 1.22
require github.com/google/go-cmp v0.7.0
GOMOD
printf 'package main\nimport ("fmt"; "github.com/google/go-cmp/cmp")\nfunc main() { fmt.Println(cmp.Equal(1, 1)) }\n' > main.go
GOFLAGS="-mod=mod -modcacherw" go run . >/dev/null
[ "$(field go bytes)" -gt 0 ] || fail "Go caches not found"
[ "$(field go prunableBytes '--older-than 1h')" = "0" ] || fail "fresh Go entries counted as old"
"$dd" clean --only go --all --yes >/dev/null
[ "$(GOFLAGS='-mod=mod -modcacherw' go run .)" = "true" ] || fail "go run failed after clean"
pass "Go caches cleared and the build works again"

step "Docker (Linux)"
if [ "$(uname -s)" = "Linux" ] && docker info >/dev/null 2>&1; then
  docker pull -q hello-world >/dev/null
  docker tag hello-world diskdiet-test:old
  docker volume create diskdiet-test-volume >/dev/null
  "$dd" clean --only docker --all --yes >/dev/null
  if docker image inspect diskdiet-test:old >/dev/null 2>&1; then fail "unused image kept"; fi
  docker volume inspect diskdiet-test-volume >/dev/null 2>&1 || fail "a volume was removed without --volumes"
  pass "unused images removed, volumes kept"
  docker volume rm diskdiet-test-volume >/dev/null
else
  pass "Docker unavailable here; skipped"
fi

step "Nothing outside the caches changed"
[ "$(cat "$sentinel")" = "precious" ] || fail "a file outside the caches was changed or removed"
pass "the file linked from inside the npm cache is intact"

printf '\nAll integration tests passed.\n'
