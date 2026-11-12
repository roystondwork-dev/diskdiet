## diskdiet

See where your developer disk space went, and get it back safely

### Synopsis

diskdiet measures the caches developer tools leave behind (npm, Yarn, pnpm,
Bun, pip, uv, Cargo, Go, Gradle, Maven, Playwright, Cypress, Xcode, Docker) and
the node_modules and target folders of projects you haven't touched, then frees
what is old: not used in 30 days by default.

Run without a command, diskdiet scans and changes nothing.

```
diskdiet [flags]
```

### Examples

```
  diskdiet                         # what's using space, and what can go
  diskdiet clean                   # free what's older than 30 days, after asking
  diskdiet clean --older-than 7d --only npm,docker
  diskdiet clean --all --dry-run   # what removing everything would free
```

### Options

```
      --all                    count everything, whatever its age
  -h, --help                   help for diskdiet
      --json                   print JSON
      --older-than string      count things unchanged for this long as old (e.g. 7d, 2w, 12h) (default "30d")
      --only strings           only these sources: npm, yarn, pnpm, bun, pip, uv, cargo, go, gradle, maven, playwright, cypress, xcode, docker, node_modules, target
      --projects stringArray   folder to search for node_modules (repeatable; default: your home folder)
      --skip strings           leave out these sources
```

### SEE ALSO

* [diskdiet clean](diskdiet_clean.md)	 - Free space: remove what's old, after showing the list and asking
* [diskdiet mcp](diskdiet_mcp.md)	 - Serve diskdiet to AI assistants (Model Context Protocol)
* [diskdiet scan](diskdiet_scan.md)	 - Show what's using space and what can be freed (changes nothing)
* [diskdiet version](diskdiet_version.md)	 - Print the diskdiet version

