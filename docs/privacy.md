# Privacy

diskdiet, its desktop app, its VS Code extension and its MCP server **collect no
personal data**. Nothing is sent to the diskdiet project or to anyone else: no
accounts, analytics, crash reports or tracking.

## What diskdiet reads

- The folders of the caches it knows (npm, Yarn, pnpm, Bun, pip, uv, Cargo, Go, Gradle,
  Maven, Playwright, Cypress, Xcode) and, to find `node_modules` and `target` folders,
  your home folder (or the folders you pass with `--projects`). It reads file sizes and
  dates, and the index files of npm's cache to know which entries are old. It does not
  read your files' contents.
- The list of running processes, to avoid cleaning a cache while its package manager is
  installing.

## What it runs and connects to

diskdiet itself makes no network connections. It runs, on your machine:

- `npm config get cache`, `pnpm store path`, `go env`, `uv cache dir` to find caches;
- `pnpm store prune`, `uv cache clean` and `docker … prune` when you clean those;
- `docker info` and `docker system df`, which talk to your local Docker.

Those tools may contact their own servers as they normally do; diskdiet adds nothing.

The **npm package** downloads the diskdiet binary from this project's GitHub releases
on first use. The **desktop app** stores its settings in a `settings.json` file in your
user settings folder, and only shows notifications on your machine.

## Contact

Questions: [open an issue](https://github.com/roystondwork-dev/diskdiet/issues/new/choose).
