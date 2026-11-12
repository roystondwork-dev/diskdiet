# Governance

diskdiet is a community open-source project released under the [MIT License](LICENSE).

## Maintainers

| Name | GitHub | Role |
|---|---|---|
| Royston D | [@roystondwork-dev](https://github.com/roystondwork-dev) | Lead maintainer |

Maintainers review and merge pull requests, triage issues, cut releases, and enforce
the [Code of Conduct](CODE_OF_CONDUCT.md).

## Decision making

- Day-to-day changes are decided through pull request review: maintainer approval plus
  green CI is required to merge.
- Changes to what diskdiet deletes or how (new sources, new removal rules, the
  safety checks in `internal/fsx` and `internal/source`) are discussed in an issue first
  and need a test showing what is and isn't removed.
- When consensus can't be reached, the lead maintainer decides.

## Becoming a maintainer

Contributors with a sustained record of high-quality contributions and reviews may be
nominated by an existing maintainer. Nominations are approved by a majority of current
maintainers.

## Compatibility promise

- The CLI's JSON output carries a `schemaVersion`; fields are only removed or changed in
  a new schema version, announced in the changelog.
- Exit codes are documented in the README and change only in a major version.
