---
description: "Why ~/.npm gets huge (often 10–40 GB) and how to shrink it without slowing down installs: remove only old entries with diskdiet."
---

# The npm cache is huge

npm keeps every package tarball and metadata response it ever downloaded in
`~/.npm/_cacache` (`%LocalAppData%\npm-cache` on Windows) and **never removes anything
on its own**. After a year or two it's commonly 10–40 GB.

## The blunt fix

```sh
npm cache clean --force
```

This deletes **everything**, so your next installs re-download every package, even the
ones you use daily.

## Remove only what you don't use

```sh
npx diskdiet clean --only npm
```

diskdiet removes entries written more than 30 days ago (change with `--older-than`),
plus old `npx` installs and logs, and keeps the rest:

- it reads npm's cache index to know when each entry was written,
- rewrites the index without the old entries,
- removes content no remaining entry points at (content shared with a newer entry
  stays).

`npm cache verify` reports a healthy cache afterwards, and packages you installed recently
still install offline.

## Check first

```sh
npx diskdiet scan --only npm          # how big, how much is old
npx diskdiet clean --only npm --dry-run
```

## Related

- [Old node_modules folders](node-modules.md), the other big npm-related space user.
- pnpm users: diskdiet runs `pnpm store prune` (see [What diskdiet cleans](../guide/sources.md#pnpm)).
