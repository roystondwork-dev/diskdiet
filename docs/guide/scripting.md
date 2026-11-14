---
description: Use diskdiet in scripts and CI with JSON output, stable exit codes and --yes.
---

# Scripting and CI

## JSON

```sh
diskdiet scan --json
diskdiet clean --dry-run --json
diskdiet clean --yes --json --only npm,pip
```

Every document has `"schemaVersion": 1`. Fields are only removed or changed in a new
schema version. A scan looks like:

```json
{
  "schemaVersion": 1,
  "olderThan": "30d",
  "bytes": 97500000000,
  "prunableBytes": 64200000000,
  "sources": [
    {
      "id": "npm", "name": "npm cache", "found": true,
      "locations": ["/Users/you/.npm"],
      "bytes": 41300000000, "items": 48210,
      "prunableBytes": 33800000000, "prunableItems": 39102
    },
    {
      "id": "node_modules", "name": "node_modules", "found": true,
      "bytes": 11200000000, "prunableBytes": 5400000000, "prunableItems": 5,
      "projects": [{ "path": "/Users/you/code/shop/node_modules", "bytes": 1900000000, "idleDays": 140 }]
    }
  ]
}
```

`estimate: true` marks sources where the tool decides what goes (pnpm, Docker), so
`prunableBytes` is the most that can be freed. `clean --json` adds `plan`, `results`
(per source: `freedBytes`, `removed`, `skipped`, `notes`, `errors`) and `freedBytes`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (including "nothing to free" and answering no) |
| 1 | Error |
| 2 | Wrong usage, such as an unknown flag or source |
| 3 | `clean` finished, but some items could not be removed |

## Free space on CI runners

On GitHub Actions, use the [diskdiet action](github-action.md):

```yaml
- uses: roystondwork-dev/diskdiet@v0.7.0 # x-release-please-version
  with:
    remove-preinstalled: android,dotnet
```

Elsewhere, run the CLI directly:

```sh
npx -y diskdiet clean --all --yes --skip node_modules,target
```

## A weekly clean on your machine

```sh
# crontab -e
0 10 * * 1  /usr/local/bin/diskdiet clean --yes --only npm,yarn,pnpm,bun,pip
```

Or use the [desktop app's](../app.md) **Clean automatically** setting.
