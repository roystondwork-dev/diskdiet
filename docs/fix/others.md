---
description: "Clean Cargo, Go, Gradle and Maven caches and Rust/Maven target folders safely: remove only what hasn't been used lately with diskdiet."
---

# Cargo, Go, Gradle, Maven

```sh
npx diskdiet scan --only cargo,go,gradle,maven,target
npx diskdiet clean --only cargo,go,gradle,maven,target
```

| Source | What diskdiet removes (unchanged for 30 days) | Comes back with |
|---|---|---|
| **Cargo** (`~/.cargo`) | downloaded `.crate` files, unpacked sources, git checkouts | the next `cargo build` |
| **target folders** | `target` of Rust (`Cargo.toml`) and Maven (`pom.xml`) projects you haven't touched | `cargo build`, `mvn package` |
| **Go** | build-cache entries not used for 30 days; module versions downloaded before then | the next `go build` |
| **Gradle** (`~/.gradle`) | old Gradle distributions, per-version caches and daemon logs | the Gradle wrapper |
| **Maven** (`~/.m2/repository`) | artifact versions downloaded before then | the next build |

Notes:

- **Go:** Go marks build-cache entries when it uses them, so "30 days" means *not used*
  for 30 days. Module folders are read-only; diskdiet handles that.
- **Gradle:** the shared dependency cache (`caches/modules-2`) is left to Gradle, which
  cleans it itself.
- **target:** a folder named `target` in a project without `Cargo.toml` or `pom.xml` is
  never touched.
- If a `cargo`, `go`, Gradle or Maven build is running, diskdiet skips that cache.

The blunt alternatives (`cargo clean` per project, `go clean -cache -modcache`, deleting
`~/.gradle`) remove everything and make the next build slow.
