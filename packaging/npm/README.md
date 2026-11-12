# diskdiet

See where your developer disk space went, and get it back safely.

```sh
npx diskdiet            # what's using space, and what can go (changes nothing)
npx diskdiet clean      # free what's older than 30 days, after showing the list
```

diskdiet measures the caches developer tools leave behind (npm, Yarn, pnpm, Bun, pip,
uv, Cargo, Go, Gradle, Maven, Playwright, Cypress, Xcode, Docker) and the
`node_modules` and `target` folders of projects you haven't touched, then removes only
what you haven't used lately.

This package downloads the diskdiet binary for your OS and CPU from the
[GitHub release](https://github.com/roystondwork-dev/diskdiet/releases) on first use,
checks it against the checksums published with the package, and caches it.

For AI assistants: `claude mcp add diskdiet -- npx -y diskdiet mcp`.

Full documentation, the desktop app and other ways to install:
<https://github.com/roystondwork-dev/diskdiet>

MIT © 2026 Royston D and diskdiet contributors
