## diskdiet scan

Show what's using space and what can be freed (changes nothing)

```
diskdiet scan [flags]
```

### Options

```
      --all                    count everything, whatever its age
  -h, --help                   help for scan
      --json                   print JSON
      --older-than string      count things unchanged for this long as old (e.g. 7d, 2w, 12h) (default "30d")
      --only strings           only these sources: npm, yarn, pnpm, bun, pip, uv, cargo, go, gradle, maven, playwright, cypress, xcode, simulators, cocoapods, swiftpm, android, homebrew, docker, node_modules, target
      --projects stringArray   folder to search for node_modules (repeatable; default: your home folder)
      --skip strings           leave out these sources
```

### SEE ALSO

* [diskdiet](diskdiet.md)	 - See where your developer disk space went, and get it back safely

