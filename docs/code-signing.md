---
description: The state of code signing for diskdiet's Windows downloads, how to check a download today, and who will approve signed releases.
---

# Code signing

!!! warning "Windows downloads are not code signed yet"
    diskdiet's Windows programs don't carry a Microsoft Authenticode signature yet, so
    SmartScreen ("Windows protected your PC") or your browser may warn before running
    them. Every download *is* signed with [Sigstore](verifying.md), so you can check it
    came from this project before you allow it; see [below](#on-windows-today).

## What will be signed

Once Windows code signing is in place, the Windows programs in each [GitHub release](https://github.com/roystondwork-dev/diskdiet/releases):

- `diskdiet.exe`, the command-line tool, inside `diskdiet_<version>_windows_<arch>.zip`;
- the desktop app's installer, `diskdiet-app_<version>_windows_amd64_setup.exe`, and the
  app inside it.

Only programs built from this repository's source code are signed. They're built by
GitHub Actions on GitHub-hosted runners from a tagged commit
([release workflow](https://github.com/roystondwork-dev/diskdiet/blob/main/.github/workflows/release.yml)),
and every release will be approved by hand before it is signed. Nothing built on a
developer's machine will ever be signed.

Today, every release file for every OS is already signed with
[Sigstore](verifying.md) and comes with SBOMs and build provenance.

## Team roles

| Role | Who |
|---|---|
| Committers and reviewers | [Members of the repository](https://github.com/roystondwork-dev/diskdiet/graphs/contributors) with write access: [roystondwork-dev](https://github.com/roystondwork-dev) |
| Approvers | Repository owner: [roystondwork-dev](https://github.com/roystondwork-dev) |

Committers may change the code; every change by anyone else is reviewed in a pull
request before it is merged. Approvers decide whether a release gets signed. Everyone
in these roles uses multi-factor authentication.

## Privacy

This program will not transfer any information to other networked systems unless
specifically requested by the user or the person installing or operating it. See the
[privacy policy](privacy.md): diskdiet collects no personal data, has no analytics or
telemetry, and makes no network connections of its own.

## On Windows today

1. Check the download is genuine: in PowerShell, `Get-FileHash <file>` must print the
   same SHA-256 as the release's `checksums.txt` (desktop app: `diskdiet-app_checksums.txt`).
   [Verifying downloads](verifying.md) also shows how to check the Sigstore signature.
2. If SmartScreen says "Windows protected your PC", click **More info**, then **Run anyway**.
3. Or skip the warning entirely: `npx diskdiet` (npm) or `pipx install diskdiet` (PyPI)
   install the command-line tool without one.

If Microsoft Defender reports a threat in a download whose checksum matches, it's a
false positive: please [tell us](https://github.com/roystondwork-dev/diskdiet/issues/new/choose)
and [report it to Microsoft](https://www.microsoft.com/wdsi/filesubmission).

## Report a problem

If you think a signed diskdiet program is malicious or was not built from this
repository, report it privately as described in the [security policy](security.md).
