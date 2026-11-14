---
description: diskdiet scan and diskdiet clean, every flag explained, with examples.
---

# Scan and clean

diskdiet has two everyday commands: **`scan`** (the default) measures and changes
nothing; **`clean`** removes what the scan reports as old, after showing the list and
asking.

## What counts as old

| Flag | Meaning |
|---|---|
| `--older-than 30d` (default) | Unchanged for this long. Accepts `12h`, `7d`, `2w`… |
| `--all` | Everything, whatever its age |

"Unchanged" means different things per cache: for npm it's when the entry was written to
the cache; for Go's build cache it's the last time Go used it; for `node_modules` and
`target` folders it's when anything at the top of the project last changed. See
[What diskdiet cleans](sources.md).

## Choosing sources

| Flag | |
|---|---|
| `--only npm,docker` | Only these sources |
| `--skip node_modules` | Everything but these |
| `--projects ~/code` | Where to look for `node_modules` and `target` folders (repeatable; default: your home folder) |
| `--keep DIR` | Leave this `node_modules` or `target` folder alone (repeatable) |
| `--trash` | Move `node_modules` and `target` folders to the Trash (Recycle Bin on Windows) instead of deleting them; see below |
| `--volumes` | Also remove unused Docker volumes (they can hold databases) |

Source names: `npm`, `yarn`, `pnpm`, `bun`, `pip`, `uv`, `cargo`, `go`, `gradle`, `maven`,
`playwright`, `cypress`, `xcode`, `simulators`, `cocoapods`, `swiftpm`, `android`, `homebrew`,
`docker`, `node_modules`, `target`.

## Confirming

| Flag | |
|---|---|
| (none) | Show the list, ask `[y/N]` |
| `-n`, `--dry-run` | Show the list, remove nothing |
| `-y`, `--yes` | Don't ask (scripts, CI) |
| `--force` | Clean even while a package manager is installing |

Without a terminal (in a script), `clean` refuses to run unless you pass `--yes` or
`--dry-run`.

## Undo: move project folders to the Trash

```sh
diskdiet clean --only node_modules --trash
```

`node_modules` and `target` folders go to the macOS Trash, the Windows Recycle Bin or the
Linux desktop Trash, so you can put any of them back. Their space is freed when you empty
the Trash, so on a full disk, clean without `--trash`. Caches are always deleted: everything
in them downloads again anyway.

## Safety rules

- What `clean` removes is exactly what it showed you.
- Every removal is checked to be inside the cache folder it belongs to; symlinks are
  never followed.
- pnpm, uv and Docker decide what is unused with their own commands
  (`pnpm store prune`, `uv cache clean`, `docker … prune`).
- If `npm install`, `pnpm add`, `yarn`, `cargo build`, `go test` or a Gradle or Maven
  build is running, that cache is skipped. A long-running `npm run dev` or an idle Gradle
  daemon doesn't count.
- Docker volumes stay unless you pass `--volumes`.

## Examples

```sh
diskdiet scan --projects ~/code --projects ~/work
diskdiet clean --older-than 2w --skip docker
diskdiet clean --only node_modules --keep ~/code/current-project/node_modules
diskdiet clean --all --only uv --yes
```

Full reference: [`diskdiet clean`](../commands/diskdiet_clean.md),
[`diskdiet scan`](../commands/diskdiet_scan.md).
