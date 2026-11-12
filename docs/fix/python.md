---
description: "Clear the pip cache and uv cache safely: remove old downloads and wheels with diskdiet, or clear uv's cache completely."
---

# pip and uv caches

## pip

pip caches every download and every wheel it builds in `~/Library/Caches/pip`,
`~/.cache/pip` or `%LocalAppData%\pip\Cache`, and keeps them forever.

```sh
npx diskdiet clean --only pip                  # entries older than 30 days
pipx run diskdiet clean --only pip             # the same, from PyPI
```

The blunt alternative, `pip cache purge`, removes everything.

## uv

uv's cache (`uv cache dir`, usually `~/.cache/uv`) can grow to many gigabytes. Its
entries point at one another, so diskdiet doesn't remove them one by one: with `--all`
it runs uv's own **`uv cache clean`**.

```sh
npx diskdiet clean --all --only uv
```

`uv cache prune` (unused entries only) is a lighter option you can run yourself.

## Virtual environments

diskdiet doesn't delete `.venv` folders: they can contain packages that aren't written
down anywhere else.
