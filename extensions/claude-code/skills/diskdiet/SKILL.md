---
name: diskdiet
description: >-
  Finds what is filling the disk on a developer machine and frees it safely with
  diskdiet: package caches (npm, Yarn, pnpm, Bun, pip, uv, Cargo, Go, Gradle, Maven),
  Playwright and Cypress downloads, Xcode DerivedData, Docker images and build cache,
  and node_modules or target folders of projects nobody has touched lately. Use when
  the user is low on disk space, a build or install fails with "no space left on
  device" (ENOSPC), or the user asks what is using space or to clean caches.
when_to_use: >-
  "my disk is full", "no space left on device", "ENOSPC", "what's using my disk space",
  "clean npm cache", "my ~/.npm is huge", "free up space", "delete old node_modules",
  "docker is using too much space", "clear package caches"
---

# Freeing disk space with diskdiet

diskdiet measures the caches and build folders developer tools leave behind and removes
only what hasn't been used lately (30 days by default). Everything it removes from a
cache downloads again when a project needs it; `node_modules` come back with an install
and `target` folders with a build.

## Which tools to use

Prefer diskdiet's MCP tools (this plugin starts them):

| Tool | Use it to |
|---|---|
| `scan_disk_usage` | measure everything and see what can be freed. Changes nothing. Options: `older_than_days` (default 30; 0 = everything), `sources`, `projects`, `include_docker_volumes` |
| `free_disk_space` | remove what the last scan reported, for the `sources` the user chose; `keep_projects` keeps individual folders |

Without the MCP tools, run the CLI. `npx -y diskdiet` works with nothing installed; use
`diskdiet` if it is on PATH.

```sh
npx -y diskdiet                          # scan (changes nothing)
npx -y diskdiet scan --json              # the same, as JSON
npx -y diskdiet clean --dry-run          # what clean would remove
npx -y diskdiet clean --only npm,go --yes   # only after the user agreed
```

## How to help

1. Run `scan_disk_usage` and tell the user the total, what can be freed, and the three
   or four biggest sources, in plain words.
2. Recommend the package caches first: removing their old entries costs at most a
   re-download.
3. Ask before including `node_modules`, `target` or Docker. Show the project folders
   that would go; offer to keep any with `keep_projects`. Never include Docker volumes
   unless the user asks: they can hold databases.
4. Call `free_disk_space` only after the user says which sources to clean. Report what
   was freed and any notes (for example that Docker Desktop's disk file may shrink
   later).

If little is old enough, a shorter `older_than_days` (such as 7) finds more; say so
rather than choosing it for the user.
