---
description: Install diskdiet with npm, pipx, uv, Go, Homebrew, Scoop, winget or Linux packages, or download the desktop app.
---

# Install

diskdiet is a single binary for macOS, Linux and Windows (x64 and arm64). Pick whatever
you already use.

## The CLI

| How | Command |
|---|---|
| **Install script** (macOS, Linux) | `curl -fsSL https://roystondwork-dev.github.io/diskdiet/install.sh \| sh` |
| **Install script** (Windows PowerShell) | `irm https://roystondwork-dev.github.io/diskdiet/install.ps1 \| iex` |
| **npm** (any OS, nothing to install first) | `npx diskdiet` |
| **PyPI** (any OS) | `pipx install diskdiet` or `uv tool install diskdiet` |
| **Go** 1.26+ | `go install github.com/roystondwork-dev/diskdiet/cmd/diskdiet@latest` |
| **Homebrew** (macOS, Linux) | `brew install roystondwork-dev/tap/diskdiet` *(coming soon)* |
| **Scoop** (Windows) | `scoop bucket add roystondwork-dev https://github.com/roystondwork-dev/scoop-bucket` then `scoop install diskdiet` *(coming soon)* |
| **winget** (Windows) | `winget install roystondwork-dev.diskdiet` *(coming soon)* |
| **Linux packages** | `.deb`, `.rpm` and `.apk` from the [latest release](https://github.com/roystondwork-dev/diskdiet/releases/latest) |
| **Archives** | `diskdiet_<version>_<os>_<arch>.tar.gz` / `.zip` from the [latest release](https://github.com/roystondwork-dev/diskdiet/releases/latest) |

Check it works:

```sh
diskdiet version
```

The npm package downloads the binary for your OS from the GitHub release on first use
and checks it against checksums published with the package; the PyPI wheels contain
the binary.

## The desktop app

From the [latest release](https://github.com/roystondwork-dev/diskdiet/releases/latest):

| OS | File |
|---|---|
| macOS (Apple silicon and Intel) | `diskdiet-app_<version>_macos_universal.dmg` |
| Windows | `diskdiet-app_<version>_windows_amd64_setup.exe` (installs for your user, no admin needed) |
| Linux | `diskdiet-app_<version>_linux_x86_64.AppImage` |

The app isn't signed by Apple or Microsoft yet, so the first launch needs one extra
step; see [the app guide](app.md#install).

## The VS Code extension

Install `diskdiet-vscode-<version>.vsix` from the
[latest release](https://github.com/roystondwork-dev/diskdiet/releases/latest) with
**Extensions → ⋯ → Install from VSIX…**. It has diskdiet built in. See
[VS Code](vscode.md).

## For AI assistants

```sh
claude mcp add diskdiet -- npx -y diskdiet mcp
```

or, in Claude Code, the plugin: `/plugin install diskdiet --marketplace roystondwork-dev/diskdiet`.
See [AI assistants (MCP)](mcp.md).

## Verify what you downloaded

Every release is signed and has SBOMs and build provenance: see
[Verifying downloads](verifying.md).
