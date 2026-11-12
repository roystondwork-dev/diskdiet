# diskdiet for VS Code

See what developer caches and old project folders use your disk, and free space
safely without leaving the editor.

- **Disk space view** in the activity bar: npm, Yarn, pnpm, Bun, pip, uv, Cargo, Go,
  Gradle, Maven, Playwright, Cypress, Xcode and Docker caches, plus the `node_modules`
  and `target` folders of projects you haven't touched, biggest first, with what can go.
- **Free Disk Space…** lets you pick what to free. Package caches are ticked by default;
  `node_modules`, `target` folders and Docker only if you tick them, and you can keep
  individual projects. It always asks before removing anything.
- **"No space left on device"?** When a command in the terminal fails with `ENOSPC`,
  diskdiet offers to free space right away.
- **Status bar and reminders**: shows how much can be freed when it passes 10 GB, and
  checks in the background every 6 hours.

diskdiet removes only what hasn't been used lately (30 days by default; change it with
**diskdiet: Set What Counts as Old…**). Everything removed from a cache downloads again
when a project needs it.

The diskdiet binary for your OS is built in; nothing else to install.

## Settings

| Setting | Default | |
|---|---|---|
| `diskdiet.olderThanDays` | `30` | What counts as old; `0` counts everything |
| `diskdiet.statusBar` | `aboveThreshold` | `always`, `aboveThreshold` or `never` |
| `diskdiet.warnAboveGB` | `10` | Notify once per session above this; `0` turns it off |
| `diskdiet.checkEveryHours` | `6` | Background checks; `0` turns them off |
| `diskdiet.detectDiskFull` | `true` | Offer to free space when a terminal command runs out of disk |
| `diskdiet.path` | | Use another diskdiet binary |

Also available as a CLI, a desktop app with a tray icon, and an MCP server for AI
assistants: <https://github.com/roystondwork-dev/diskdiet>.
