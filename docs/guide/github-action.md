---
description: Free disk space on GitHub Actions runners with the diskdiet action - package caches, Docker build cache and, optionally, preinstalled SDKs (Android, .NET, Haskell, CodeQL).
---

# GitHub Action

Free disk space on a CI runner in one step:

```yaml
- uses: roystondwork-dev/diskdiet@v0.7.0 # x-release-please-version
```

By default it removes **everything in every package cache** diskdiet knows (npm, Yarn,
pnpm, pip, Go, Gradle, Maven, Cargo, Docker build cache and unused images…) and shows
how much space it gained in the job summary. It **never touches `node_modules` or
`target` folders**, so the project you're building stays intact.

## Getting 20–30 GB more on GitHub-hosted runners

GitHub's Linux runners come with large SDKs most jobs never use. Remove the ones you
don't need:

```yaml
- uses: roystondwork-dev/diskdiet@v0.7.0 # x-release-please-version
  with:
    remove-preinstalled: android,dotnet,haskell,codeql
```

| Value | Removes | About |
|---|---|---|
| `android` | `/usr/local/lib/android` | 9–12 GB |
| `dotnet` | `/usr/share/dotnet` | 1–3 GB |
| `haskell` | `/opt/ghc`, `~/.ghcup` | 3–6 GB |
| `codeql` | the CodeQL bundle in the tool cache | 1–5 GB |
| `swift` | `/usr/share/swift` | 1–2 GB |
| `boost` | `/usr/local/share/boost` | ~1 GB |
| `tool-cache` | the whole hosted tool cache (Python, Node, Go, Ruby… used by `setup-*` actions) | 5–10 GB |
| `all` | all of the above | 20–30 GB |

This only runs on Linux GitHub-hosted runners; on self-hosted runners it's skipped with a
warning. Remove `tool-cache` only if no later step uses `actions/setup-node`,
`setup-python`, `setup-go` and similar.

## Inputs

| Input | Default | |
|---|---|---|
| `older-than` | *(empty: everything)* | Remove only what's unchanged for this long, e.g. `7d` (useful on self-hosted runners) |
| `only` | *(empty)* | Only these sources, e.g. `npm,pip,docker` |
| `skip` | `node_modules,target` | Sources to leave alone |
| `docker-volumes` | `false` | Also remove unused Docker volumes |
| `remove-preinstalled` | *(empty)* | See above |
| `dry-run` | `false` | Show what would be removed, remove nothing |
| `version` | `latest` | A diskdiet release, e.g. `0.6.3` |

## Outputs

| Output | |
|---|---|
| `freed-bytes` | Bytes freed from caches |
| `free-before` | Free bytes on the workspace's disk before |
| `free-after` | Free bytes after |

```yaml
- uses: roystondwork-dev/diskdiet@v0.7.0 # x-release-please-version
  id: diskdiet
  with:
    remove-preinstalled: android,dotnet
- run: echo "Now ${{ steps.diskdiet.outputs.free-after }} bytes free"
```

## Where to put it

At the start of a job that runs out of space, before big downloads or builds. If you use
`actions/cache` or `setup-*` caching, put diskdiet before them, or exclude their caches with
`skip`, so it doesn't remove what they just restored.

## Self-hosted runners

Caches pile up across jobs on self-hosted runners. A scheduled job keeps them in check
without slowing every build:

```yaml
on:
  schedule:
    - cron: "0 3 * * 0"
jobs:
  diskdiet:
    runs-on: self-hosted
    steps:
      - uses: roystondwork-dev/diskdiet@v0.7.0 # x-release-please-version
        with:
          older-than: 7d
```

The diskdiet binary is downloaded from the release and checked against its checksums.
Linux, macOS and Windows runners are supported.
