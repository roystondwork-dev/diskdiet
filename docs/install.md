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
Windows downloads aren't code signed yet: see [Code signing](code-signing.md) for how to
check one and get past the warning.

## Uninstall

diskdiet changes nothing outside the files below, so removing them removes it completely.

**The CLI**, the way you installed it:

| Installed with | Remove with |
|---|---|
| Install script (macOS, Linux) | `rm ~/.local/bin/diskdiet` (or the `DISKDIET_INSTALL_DIR` you chose) |
| Install script (Windows) | Delete `%LOCALAPPDATA%\Programs\diskdiet`, then remove that folder from your user `Path` (Start → "Edit environment variables for your account") |
| npm | Nothing to do for `npx`; `npm uninstall -g diskdiet` if you installed it globally |
| PyPI | `pipx uninstall diskdiet` or `uv tool uninstall diskdiet` |
| Go | `rm "$(go env GOPATH)/bin/diskdiet"` |
| Linux packages | `sudo apt remove diskdiet`, `sudo dnf remove diskdiet` or `sudo apk del diskdiet` |

**The desktop app.** First, in the tray menu, untick *Start at login* if you turned it on.

- **Windows:** Settings → Apps → Installed apps → diskdiet → Uninstall.
- **macOS:** drag diskdiet from Applications to the Trash.
- **Linux:** delete the `.AppImage` file.

Its settings file can then be deleted too: `diskdiet/settings.json` in your user settings
folder (`%APPDATA%` on Windows, `~/Library/Application Support` on macOS, `~/.config` on
Linux).

**The VS Code extension:** Extensions → diskdiet → Uninstall.
