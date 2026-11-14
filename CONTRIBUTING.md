# Contributing to diskdiet

Thanks for helping! Issues and pull requests are welcome.

## Setup

You need Go (see `go.mod` for the version) and, for linting,
[golangci-lint](https://golangci-lint.run) v2.

```sh
make build     # ./bin/diskdiet
make test      # tests with the race detector
make lint      # golangci-lint and actionlint
make cover     # tests with the coverage gate
make fuzz      # every fuzz target (FUZZTIME=30s each); CI runs them for 20 s
make mutation  # mutation testing of the safety-critical packages (slow; weekly in CI)
```

`scripts/integration.sh` fills real npm, Yarn, pnpm, pip, Go and Docker caches and
cleans them for real, under a temporary fake home folder. CI runs it on every OS; on
your own machine it needs `DISKDIET_INTEGRATION_LOCAL=1`.

Optional: `lefthook install` sets up git hooks that format code, check commit
messages and run lint and tests before pushing.

## Ground rules

- **Safety first.** diskdiet deletes files. Every removal goes through
  `fsx.Remove`, which refuses paths outside the cache folder. A scan plans what to
  remove and a clean removes exactly that plan. Keep it that way.
- **Use the tool's own command when there is one** (`pnpm store prune`,
  `docker ... prune`). Write our own removal logic only for what the tool can't do,
  like age-based pruning.
- **Test on every OS.** CI runs on Linux, macOS and Windows; use `filepath.Join`,
  not `/`.
- **Commit messages** follow [Conventional Commits](https://www.conventionalcommits.org)
  (`feat(npm): …`, `fix(docker): …`). Releases and the changelog are generated
  from them.

## Adding a cache

Implement `source.Source` in `internal/source` (see `dir.go` for caches made of
package folders), add it to `source.All` and `source.IDs`, and test it with
`t.TempDir()` and a fake `runner.Runner`.
