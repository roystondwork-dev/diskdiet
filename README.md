<h1 align="center">diskdiet</h1>

<p align="center">
  <b>See where your developer disk space went, and get it back safely.</b><br>
  npm, Yarn, pnpm, Bun, pip, uv, Cargo, Go, Gradle, Maven, Playwright, Cypress, Xcode, Docker,<br>
  and forgotten node_modules and target folders. One binary for macOS, Linux and Windows.
</p>

<p align="center">
  <a href="https://github.com/roystondwork-dev/diskdiet/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/roystondwork-dev/diskdiet/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/roystondwork-dev/diskdiet/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/roystondwork-dev/diskdiet"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-blue.svg"></a>
</p>

## Why

Package managers never forget. After a year of `npm install`, the npm cache alone
can pass 40 GB, and Docker images, the pnpm store and old projects' `node_modules`
add more. Most cleaners either wipe everything, so your next installs crawl, or only
handle one tool.

diskdiet shows the whole picture first, then removes only what you haven't used
lately: by default, anything unchanged for 30 days.

<p align="center"><img alt="diskdiet scanning and cleaning (made-up demo data)" src="docs/assets/demo.gif" width="800"></p>

## Desktop app

Prefer clicking? The diskdiet app shows the same breakdown in a window: how much
can be freed, a bar for every cache, and the `node_modules` of each project you
haven't touched, which you can untick to keep. Pick an age (7, 30 or 90 days, or
everything), review the list, and free it in one click.

It also stays in your menu bar (or system tray) after you close the window, checks
every few hours, and notifies you when more than 10 GB could be freed. If you want, it
can clean old package-cache entries automatically every day or week, but never
`node_modules` or Docker ([details](docs/app.md#the-menu-bar--tray-icon)).

Download it from the [latest release](https://github.com/roystondwork-dev/diskdiet/releases/latest):

| OS | File |
|---|---|
| macOS (Apple silicon and Intel) | `diskdiet-app_<version>_macos_universal.dmg` |
| Windows | `diskdiet-app_<version>_windows_amd64_setup.exe` (installs for your user, no admin needed) |
| Linux | `diskdiet-app_<version>_linux_x86_64.AppImage` |

The app isn't signed by Apple or Microsoft yet, so the first launch needs one extra
step ([details](docs/app.md)):

- **macOS:** open the app once, then go to **System Settings → Privacy & Security** and
  click **Open Anyway**.
- **Windows:** on the SmartScreen warning, click **More info → Run anyway**.
- **Linux:** `chmod +x diskdiet-app_*.AppImage` and run it.

## AI assistants (MCP)

Let Claude Code, Cursor, VS Code Copilot and other assistants see what is using disk
space and free it when you ask:

```sh
claude mcp add diskdiet -- diskdiet mcp
```

`scan_disk_usage` changes nothing; `free_disk_space` removes only what the scan
showed, for the sources you choose, and asks you to confirm
([details](docs/mcp.md)).

### Claude Code plugin

The plugin adds the MCP tools and a skill that tells Claude when to use them, so "my
disk is full" or "no space left on device" gets diagnosed and fixed in the
conversation, asking before anything is removed:

```
/plugin install diskdiet --marketplace roystondwork-dev/diskdiet
```

## Usage

```sh
diskdiet                                  # scan: what's using space, what can go (changes nothing)
diskdiet clean                            # free what's older than 30 days, after showing the list
diskdiet clean --older-than 7d            # be more aggressive
diskdiet clean --only npm,docker          # just these
diskdiet clean --skip node_modules
diskdiet clean --all --dry-run            # what removing everything would free
diskdiet scan --projects ~/code           # look for node_modules only under ~/code
diskdiet scan --json                      # for scripts
```

| Flag | |
|---|---|
| `--older-than 30d` | What counts as old: `12h`, `7d`, `2w`… |
| `--all` | Everything, whatever its age |
| `--only`, `--skip` | `npm`, `yarn`, `pnpm`, `bun`, `pip`, `uv`, `cargo`, `go`, `gradle`, `maven`, `playwright`, `cypress`, `xcode`, `docker`, `node_modules`, `target` |
| `--projects DIR` | Where to look for `node_modules` (repeatable; default: your home folder) |
| `-n`, `--dry-run` | Show what `clean` would remove, remove nothing |
| `-y`, `--yes` | Don't ask (for scripts and CI) |
| `--volumes` | Also remove unused Docker volumes |
| `--force` | Clean even while a package manager is installing |
| `--json` | Machine-readable output |

## What it cleans, and how

| | Where | What `clean` removes |
|---|---|---|
| **npm** | `~/.npm` (or `npm config get cache`) | Cached packages and metadata older than the cutoff; old `npx` installs and logs. Content another, newer entry still uses is kept. |
| **Yarn** | Yarn 1 cache, Yarn 2+ global cache | Package folders and zips older than the cutoff |
| **pnpm** | `pnpm store path` | Packages no project uses, via `pnpm store prune`; old metadata |
| **Bun** | `~/.bun/install/cache` | Package folders older than the cutoff |
| **pip** | `pip` cache (`PIP_CACHE_DIR`) | Cached downloads and built wheels older than the cutoff |
| **uv** | `uv cache dir` | The whole cache via `uv cache clean`, only with `--all` (uv's entries depend on each other) |
| **Cargo** | `~/.cargo` (`CARGO_HOME`) | Downloaded `.crate` files, unpacked sources and git checkouts older than the cutoff |
| **Go** | `GOCACHE`, `GOMODCACHE` | Build cache entries not used for the cutoff (Go records their last use); module versions downloaded before it |
| **Gradle** | `~/.gradle` (`GRADLE_USER_HOME`) | Downloaded Gradle distributions and per-version caches and logs older than the cutoff; Gradle manages its shared dependency cache itself |
| **Maven** | `~/.m2/repository` | Artifact versions downloaded before the cutoff |
| **Playwright** | `ms-playwright` cache | Older browser revisions; the newest of each browser always stays |
| **Cypress** | Cypress cache | Older Cypress versions; the newest always stays |
| **Xcode** (macOS) | `~/Library/Developer/Xcode` | DerivedData and iOS/watchOS/tvOS DeviceSupport older than the cutoff; never Archives |
| **Docker** | your current docker context | Unused images, stopped containers and build cache older than the cutoff; volumes only with `--volumes` ([details](docs/docker.md)) |
| **node_modules** | projects under your home folder | `node_modules` of projects where nothing has changed for the cutoff |
| **target** | Rust (`Cargo.toml`) and Maven (`pom.xml`) projects under your home folder | Their `target` build folders, for projects where nothing has changed for the cutoff |

Anything removed from a cache is downloaded again the next time a project needs it.
A removed `node_modules` comes back with `npm install` (or `yarn`, `pnpm i`, `bun i`).

## Safety

- **Scan first, always.** `clean` shows exactly what it will remove and asks before
  removing anything. What it removes is what it showed you.
- **Never outside a cache.** Every removal is checked to be inside the cache folder
  it belongs to; symlinks are never followed.
- **The tool's own command where there is one.** pnpm and Docker decide what is
  unused, so nothing a project or a running container needs goes away.
- **Not under a running install or build.** If `npm install`, `pnpm add`, `yarn`,
  `cargo build`, `go test`, a Gradle or Maven build or similar is running, diskdiet
  skips that cache (`--force` overrides). A running `npm run dev` or an idle Gradle
  daemon doesn't count.
- **Volumes stay** unless you add `--volumes`.

## Install

```sh
# npm (any OS): run it without installing
npx diskdiet

# PyPI (any OS)
pipx install diskdiet          # or: uv tool install diskdiet

# Go 1.26+
go install github.com/roystondwork-dev/diskdiet/cmd/diskdiet@latest

# Homebrew (macOS, Linux)
brew install roystondwork-dev/tap/diskdiet

# Scoop (Windows)
scoop bucket add roystondwork-dev https://github.com/roystondwork-dev/scoop-bucket
scoop install diskdiet

# winget (Windows)
winget install roystondwork-dev.diskdiet
```

Archives, `.deb`, `.rpm` and `.apk` packages for amd64 and arm64 are on the
[releases page](https://github.com/roystondwork-dev/diskdiet/releases). The checksum file
is signed with cosign (keyless) and releases include SBOMs and build provenance.

## Demo mode

`DISKDIET_DEMO=1 diskdiet` (or the app started with `DISKDIET_DEMO=1`) shows made-up
caches and removes nothing: for trying diskdiet, screenshots and recordings.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (including "nothing to free" and answering no) |
| 1 | Error |
| 2 | Wrong usage, such as an unknown flag or source |
| 3 | `clean` finished, but some items could not be removed |

## Roadmap

- ~~**v0.2**: a desktop app for macOS, Windows and Linux~~
- ~~**v0.3**: a menu bar / tray icon that warns when caches grow, with optional scheduled pruning~~
- ~~**v0.4**: pip, uv, Cargo, Go, Gradle, Maven, Playwright, Cypress, Xcode, target folders~~
- ~~**v0.5**: an MCP server for AI assistants~~
- ~~**v0.6**: npm and PyPI packages, the MCP Registry, a Claude Code plugin, winget, demo mode~~
- **Next**: a VS Code extension

Ideas and bug reports are welcome in [issues](https://github.com/roystondwork-dev/diskdiet/issues).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Please follow the [Code of Conduct](CODE_OF_CONDUCT.md)
and report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © 2026 Royston D and diskdiet contributors
