## diskdiet clean

Free space: remove what's old, after showing the list and asking

### Synopsis

clean removes what diskdiet scan reports as old enough:

  npm, Yarn, Bun     cached packages not changed in --older-than (default 30d)
  pnpm               packages no project uses (pnpm store prune), and old metadata
  Docker             unused images, stopped containers and build cache older than
                     --older-than; unused volumes only with --volumes
  pip, Cargo, Go     cached downloads and builds not changed in --older-than
  Gradle, Maven      downloaded Gradle versions and caches; Maven artifacts
  uv                 the whole cache (uv cache clean), only with --all
  Playwright,        older browser and app versions; the newest always stays
  Cypress
  Xcode              DerivedData and device support files (never Archives)
  node_modules,      in projects where nothing has changed for --older-than
  target (Rust, Maven)

Everything removed from a cache is downloaded again when a project needs it.
It shows what it will remove and asks first, unless --yes is given.

```
diskdiet clean [flags]
```

### Examples

```
  diskdiet clean
  diskdiet clean --older-than 7d --only npm
  diskdiet clean --all --skip node_modules --yes
```

### Options

```
      --all                    count everything, whatever its age
  -n, --dry-run                show what would be removed, and remove nothing
      --force                  clean even while a package manager is installing
  -h, --help                   help for clean
      --json                   print JSON
      --keep stringArray       leave this node_modules or target folder alone (repeatable)
      --older-than string      count things unchanged for this long as old (e.g. 7d, 2w, 12h) (default "30d")
      --only strings           only these sources: npm, yarn, pnpm, bun, pip, uv, cargo, go, gradle, maven, playwright, cypress, xcode, docker, node_modules, target
      --projects stringArray   folder to search for node_modules (repeatable; default: your home folder)
      --skip strings           leave out these sources
      --volumes                also remove unused Docker volumes (they can hold databases)
  -y, --yes                    don't ask before removing
```

### SEE ALSO

* [diskdiet](diskdiet.md)	 - See where your developer disk space went, and get it back safely

