---
description: "Find and delete node_modules folders of projects you haven't touched (like npkill), across your whole home folder, safely."
---

# Old node_modules folders

Every JavaScript project you've ever cloned keeps its `node_modules`, often 200 MB–2 GB
each. Forty old projects can easily hold 10–20 GB.

## Find them

```sh
npx diskdiet scan --only node_modules
```

diskdiet searches your home folder (skipping hidden folders and `~/Library`), and
reports how many `node_modules` folders there are and which projects haven't changed for
30 days.

## Remove the old ones

```sh
npx diskdiet clean --only node_modules
```

You'll see each folder, its size and how long the project has been untouched, then be
asked:

```text
node_modules, biggest first:
  ~/code/old-landing/node_modules      1.2 GB   unchanged for 10 months
  ~/code/hackathon-2025/node_modules   900 MB   unchanged for 8 months
  ...
Free about 3.5 GB? [y/N]
```

Bringing one back is just `npm install` (or `pnpm i`, `yarn`, `bun i`) in that project.

## Want an undo?

```sh
npx diskdiet clean --only node_modules --trash
```

moves the folders to the Trash (Recycle Bin on Windows) instead of deleting them, so you
can put one back. The space is freed when you empty the Trash.

## Keep some

```sh
npx diskdiet clean --only node_modules --keep ~/code/shop/node_modules
npx diskdiet clean --only node_modules --projects ~/code      # only look under ~/code
npx diskdiet clean --only node_modules --older-than 90d       # only really old projects
```

In the [desktop app](../app.md) and [VS Code](../vscode.md) you untick the projects to
keep.

## Compared with npkill

npkill finds `node_modules` under the current folder and lets you delete them one by
one. diskdiet also judges projects by when you last touched them, covers your whole home
folder in one go, lists Rust and Maven `target` folders too, and handles every other
cache (npm, Docker, Gradle…) in the same run.
