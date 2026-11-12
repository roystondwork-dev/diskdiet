---
description: "Fix \"no space left on device\" (ENOSPC) on a developer machine: find what's filling the disk and free it safely with diskdiet."
---

# Disk full? "No space left on device"

When a build, install or `docker pull` fails with

```text
ENOSPC: no space left on device
write /tmp/...: no space left on device
There is not enough space on the disk.
```

the usual cause on a developer machine isn't your files: it's **caches and build
folders that tools never clean up**. npm, Docker, Gradle, Go and old projects'
`node_modules` easily take 50–100 GB.

## See what's taking the space

```sh
npx diskdiet
```

Nothing to install, and it changes nothing. You get every cache, biggest first, with how
much hasn't been used for 30 days:

```text
WHAT           SIZE      CAN FREE           WHERE
npm cache      41.3 GB   33.8 GB            ~/.npm
Docker         14.6 GB   up to 9.2 GB       docker (Docker Desktop)
node_modules   11.2 GB   5.4 GB (5 of 41)   41 folders under ~
```

## Free it

```sh
npx diskdiet clean
```

It shows exactly what it will remove and asks first. Removed cache entries download
again when a project needs them.

Need more? `--older-than 7d` counts more as old; `--all` counts everything.

## By cause

- [The npm cache is huge](npm-cache.md)
- [Old node_modules folders](node-modules.md)
- [Docker uses too much disk](docker.md)
- [pip and uv caches](python.md)
- [Xcode DerivedData and simulators](xcode.md)
- [Cargo, Go, Gradle, Maven](others.md)
- [Playwright and Cypress downloads](browsers.md)

!!! tip "In VS Code"
    The [diskdiet extension](../vscode.md) notices "no space left on device" in the
    terminal and offers to free space right there.
