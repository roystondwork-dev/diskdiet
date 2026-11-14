---
description: Every cache diskdiet knows (npm, Yarn, pnpm, Bun, pip, uv, Cargo, Go, Gradle, Maven, Playwright, Cypress, Xcode, Docker, node_modules, target), where it lives, and what diskdiet removes.
---

# What diskdiet cleans

For each source: where diskdiet looks, what it removes, and why that's safe. Everything
removed from a cache is downloaded or rebuilt the next time a project needs it.

## JavaScript

### npm

- **Where:** `~/.npm` on macOS and Linux, `%LocalAppData%\npm-cache` on Windows, or
  whatever `npm config get cache` (or `npm_config_cache`) says.
- **Removes:** cached packages and metadata written before the cutoff, `npx` installs
  older than it, and old logs. Content still used by a newer entry is kept.
- **How:** diskdiet reads npm's cache index (`_cacache/index-v5`) to know when each
  entry was written, rewrites the index without old entries, then removes content no
  remaining entry points at. `npm cache verify` reports a healthy cache afterwards.

### Yarn

- **Where:** Yarn 1's cache (`~/Library/Caches/Yarn/v6`, `~/.cache/yarn/v6`,
  `%LocalAppData%\Yarn\Cache\v6`, or `YARN_CACHE_FOLDER`) and Yarn 2+'s global cache
  (`~/.yarn/berry/cache`).
- **Removes:** package folders and zips older than the cutoff.

### pnpm

- **Where:** the store (`pnpm store path`) and pnpm's metadata cache.
- **Removes:** packages no project uses, by running **`pnpm store prune`** (age doesn't
  apply), plus metadata older than the cutoff.
- **Why it works this way:** projects' `node_modules` hard-link files from the store, so
  deleting store files a project uses would free nothing and break it. diskdiet's
  estimate counts only files with no other links.

### Bun

- **Where:** `~/.bun/install/cache` (or `BUN_INSTALL_CACHE_DIR`).
- **Removes:** package folders older than the cutoff.

### node_modules

- **Where:** projects under your home folder (or `--projects`); hidden folders and
  `~/Library` are skipped.
- **Removes:** the `node_modules` of projects where nothing at the top level has changed
  for the cutoff. `npm install` (or `pnpm i`, `yarn`, `bun i`) brings it back.
- **Keep one:** `--keep ~/code/project/node_modules`, or untick it in the app or VS Code.

## Python

### pip

- **Where:** `~/Library/Caches/pip`, `~/.cache/pip`, `%LocalAppData%\pip\Cache`, or
  `PIP_CACHE_DIR`.
- **Removes:** cached downloads and built wheels older than the cutoff.

### uv

- **Where:** `uv cache dir` (usually `~/.cache/uv`).
- **Removes:** the whole cache with **`uv cache clean`**, only with `--all`. uv's entries
  point at one another, so diskdiet doesn't remove them one by one.

## Rust

### Cargo

- **Where:** `~/.cargo` (or `CARGO_HOME`): `registry/cache`, `registry/src`,
  `git/checkouts`, `git/db`.
- **Removes:** downloaded `.crate` files, unpacked sources and git checkouts older than
  the cutoff.

### target folders

- **Where:** projects with a `Cargo.toml` or `pom.xml` under your home folder. A folder
  called `target` anywhere else is never touched.
- **Removes:** `target` of projects untouched for the cutoff; `cargo build` or
  `mvn package` recreates it.

## Go

- **Where:** `go env GOCACHE` and `go env GOMODCACHE`.
- **Removes:** build-cache entries not used for the cutoff (Go marks entries when it uses
  them, so age means *last use*), and module versions downloaded before it. Read-only
  module folders are handled.

## Java

### Gradle

- **Where:** `~/.gradle` (or `GRADLE_USER_HOME`).
- **Removes:** downloaded Gradle distributions (`wrapper/dists`) and per-version caches
  and daemon logs older than the cutoff. Gradle's shared dependency cache is left to
  Gradle, which cleans it itself.

### Maven

- **Where:** `~/.m2/repository`.
- **Removes:** artifact versions downloaded before the cutoff.

## Browser testing

### Playwright

- **Where:** `ms-playwright` in your cache folder (or `PLAYWRIGHT_BROWSERS_PATH`).
- **Removes:** older browser revisions. **The newest of each browser always stays**, so
  your tests keep running.

### Cypress

- **Where:** the Cypress cache (or `CYPRESS_CACHE_FOLDER`).
- **Removes:** older Cypress versions; the newest always stays.

## Mobile

### Android SDK

- **Where:** `ANDROID_HOME` / `ANDROID_SDK_ROOT`, or Android Studio's default
  (`~/Library/Android/sdk`, `~/Android/Sdk`, `%LocalAppData%\Android\Sdk`).
- **Removes:** system images no emulator uses (diskdiet reads every AVD's `config.ini`),
  and NDK and build-tools versions other than the newest, when older than the cutoff.
  Emulators are never touched. See [Android SDK](../fix/android.md).

### CocoaPods

- **Where:** CocoaPods' download cache (`~/Library/Caches/CocoaPods`).
- **Removes:** pod versions not changed for the cutoff. Projects' `Pods` folders stay.

### Swift Package Manager

- **Where:** `~/Library/Caches/org.swift.swiftpm`.
- **Removes:** cached repositories and binary artifacts older than the cutoff.

## Apple

### Xcode simulators (macOS)

- **Where:** `~/Library/Developer/CoreSimulator`.
- **Removes:** simulators for runtimes that are no longer installed (with Xcode's own
  `xcrun simctl delete unavailable`, whatever their age) and simulator caches older than
  the cutoff. Simulators you can still run are never touched.

### Xcode (macOS)

- **Where:** `~/Library/Developer/Xcode`.
- **Removes:** `DerivedData` (build products) and `iOS/watchOS/tvOS DeviceSupport`
  folders older than the cutoff. **Archives are never touched.**

## Homebrew

- **Where:** `brew --cache` (`~/Library/Caches/Homebrew`, `~/.cache/Homebrew`, or
  `HOMEBREW_CACHE`).
- **Removes:** downloaded bottles and source archives older than the cutoff, and the
  shortcuts to them. Installed software is never touched; `brew` downloads a bottle again
  if it needs one.

## Docker

- **Where:** your current docker context (Docker Desktop, OrbStack, Colima, Rancher
  Desktop, Docker Engine).
- **Removes:** unused images, stopped containers and build cache older than the cutoff,
  using Docker's own `prune` commands. Unused volumes only with `--volumes`.
- See [Docker details](../docker.md), including why Docker Desktop's disk may not shrink
  right away.

---

Missing a cache you'd like diskdiet to know?
[Suggest it](https://github.com/roystondwork-dev/diskdiet/issues/new/choose).
