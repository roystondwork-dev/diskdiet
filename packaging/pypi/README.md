# diskdiet

See where your developer disk space went, and get it back safely.

```sh
pipx install diskdiet       # or: uv tool install diskdiet / pip install diskdiet

diskdiet                    # what's using space, and what can go (changes nothing)
diskdiet clean              # free what's older than 30 days, after showing the list
diskdiet clean --only pip,uv --older-than 7d
```

diskdiet measures the caches developer tools leave behind (pip, uv, npm, Yarn, pnpm,
Bun, Cargo, Go, Gradle, Maven, Playwright, Cypress, Xcode, Docker) and the
`node_modules` and `target` folders of projects you haven't touched, then removes only
what you haven't used lately.

Works on macOS, Linux and Windows (x64 and arm64). This package installs the native
`diskdiet` binary; there's no Python code and nothing is downloaded at runtime.

Also available as a desktop app, through npm (`npx diskdiet`), Homebrew and Scoop, and
as an MCP server for AI assistants (`diskdiet mcp`).

- Documentation: https://github.com/roystondwork-dev/diskdiet
- Source (MIT): https://github.com/roystondwork-dev/diskdiet
