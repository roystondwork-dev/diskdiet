# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| latest minor release (`0.x`) | ✅ |
| older releases | ❌ — please upgrade |

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Use GitHub's private vulnerability reporting:
<https://github.com/roystondwork-dev/diskdiet/security/advisories/new>

Include as much as you can:

- affected version (`diskdiet version --json`), OS, and whether it was the CLI, the
  desktop app, the VS Code extension or the MCP server;
- a description of the issue and its impact;
- steps or a proof of concept to reproduce it.

You can expect:

- an acknowledgement within **3 business days**,
- an initial assessment within **10 business days**,
- coordinated disclosure: we will agree on a release date with you and credit you in
  the advisory unless you prefer to remain anonymous.

## Scope notes

diskdiet deletes files. Its promise is that it removes only what a scan showed, only
inside the cache or project folder it belongs to, and never follows symlinks. Issues we
especially want to hear about:

- ways to make diskdiet remove anything outside a cache folder or a reported
  `node_modules`/`target` folder: symlinks, path traversal, crafted cache index files
  (for example npm's `_cacache/index-v5`), environment variables such as
  `npm_config_cache`, `CARGO_HOME` or `GOMODCACHE`, or race conditions between scan and
  clean;
- ways for `diskdiet clean`, the app, the VS Code extension or the MCP server's
  `free_disk_space` to remove more than the user confirmed, or to skip the
  confirmation;
- the desktop app's window opening or revealing paths that weren't part of the last
  scan;
- the npm or PyPI package installing a binary other than the signed release's;
- injection through file names or command output shown in the CLI, app or extension.

## Verifying releases

Each release's `checksums.txt`, which covers every CLI artifact, is signed with
[cosign](https://github.com/sigstore/cosign) (keyless), and releases ship with SBOMs and
SLSA build provenance; the desktop app and the VS Code extension carry provenance too.
See [docs/verifying.md](docs/verifying.md) for the commands.
