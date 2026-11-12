---
description: Free developer disk space in two commands with diskdiet scan and diskdiet clean.
---

# Quickstart

## 1. See where the space went

```sh
diskdiet
```

```text
WHAT                  SIZE      CAN FREE           WHERE
npm cache             41.3 GB   33.8 GB            ~/.npm
Docker                14.6 GB   up to 9.2 GB       docker (Docker Desktop)
node_modules          11.2 GB   5.4 GB (5 of 41)   41 folders under ~
Go caches             9.2 GB    4.7 GB             ~/Library/Caches/go-build, ~/go/pkg/mod
...
Total 97.5 GB, of which about 64.2 GB can be freed (older than 30d).
```

This changes nothing. **CAN FREE** is what hasn't been used for 30 days.

## 2. Free it

```sh
diskdiet clean
```

diskdiet lists exactly what it will remove (including each `node_modules` folder and how
long its project has been untouched) and asks **Free about 64.2 GB? [y/N]**.

Everything removed from a cache is downloaded again the next time a project needs it;
a removed `node_modules` comes back with `npm install`.

## 3. Adjust

```sh
diskdiet clean --older-than 7d            # more aggressive
diskdiet clean --only npm,docker          # just these
diskdiet clean --skip node_modules        # everything but these
diskdiet clean --keep ~/code/shop/node_modules
diskdiet clean --all --dry-run            # what removing everything would free
```

Next: [what each cache is and what gets removed](guide/sources.md), or the
[desktop app](app.md) if you prefer clicking.
