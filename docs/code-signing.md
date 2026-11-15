---
description: How diskdiet's Windows programs are code signed, who can approve a signature, and what the signed programs do with your data.
---

# Code signing policy

Free code signing provided by [SignPath.io](https://signpath.io), certificate by
[SignPath Foundation](https://signpath.org).

!!! note "Status"
    diskdiet has applied to the SignPath Foundation. Until signing is switched on, the
    Windows downloads are **not yet signed**, so Windows may warn before running them;
    [check a download](verifying.md) first. This page will say from which version on
    they are signed.

## What is signed

The Windows programs in each [GitHub release](https://github.com/roystondwork-dev/diskdiet/releases):

- `diskdiet.exe`, the command-line tool, inside `diskdiet_<version>_windows_<arch>.zip`;
- the desktop app's installer, `diskdiet-app_<version>_windows_amd64_setup.exe`, and the
  app inside it.

Only programs built from this repository's source code are signed. They're built by
GitHub Actions on GitHub-hosted runners from a tagged commit
([release workflow](https://github.com/roystondwork-dev/diskdiet/blob/main/.github/workflows/release.yml)),
and every release is approved by hand before it is signed. Nothing built on a
developer's machine is ever signed.

macOS and Linux downloads, and every release file, are also signed with
[Sigstore](verifying.md), independently of Windows code signing.

## Team roles

| Role | Who |
|---|---|
| Committers and reviewers | [Members of the repository](https://github.com/roystondwork-dev/diskdiet/graphs/contributors) with write access: [roystondwork-dev](https://github.com/roystondwork-dev) |
| Approvers | Repository owner: [roystondwork-dev](https://github.com/roystondwork-dev) |

Committers may change the code; every change by anyone else is reviewed in a pull
request before it is merged. Approvers decide whether a release gets signed. Everyone
in these roles uses multi-factor authentication for GitHub and SignPath.

## Privacy

This program will not transfer any information to other networked systems unless
specifically requested by the user or the person installing or operating it. See the
[privacy policy](privacy.md): diskdiet collects no personal data, has no analytics or
telemetry, and makes no network connections of its own.

## Report a problem

If you think a signed diskdiet program is malicious or was not built from this
repository, report it privately as described in the [security policy](security.md).
