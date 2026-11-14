---
description: "GitHub Actions runner out of disk space (\"No space left on device\")? Free 20–30 GB in one step with the diskdiet action."
---

# GitHub Actions: no space left on device

GitHub-hosted runners start with about 14–20 GB free. Big Docker builds, Android or ML
dependencies, or a few large `npm install`s fill that quickly, and the job fails with
`No space left on device` or `ENOSPC`.

Add one step at the start of the job:

```yaml
- uses: roystondwork-dev/diskdiet@v0.7.0 # x-release-please-version
  with:
    remove-preinstalled: android,dotnet,haskell,codeql
```

That removes the package and Docker caches on the runner and the preinstalled SDKs your
job doesn't use, typically **20–30 GB**, and writes before/after free space to the job
summary. Your checked-out project, including its `node_modules`, isn't touched.

Details, inputs and self-hosted runners: [GitHub Action](../guide/github-action.md).
