# diskdiet for VS Code

The extension brings diskdiet into the editor: a **Disk space** view, a status bar
item, background checks, and a fix offer when a command in the terminal runs out of
disk. The diskdiet binary for your OS is built into the extension.

## Install

Search **diskdiet** in the Extensions view, or install
`diskdiet-vscode-<version>.vsix` from the
[latest release](https://github.com/roystondwork-dev/diskdiet/releases/latest)
(**Extensions → ⋯ → Install from VSIX…**). VSCodium, Cursor and Windsurf use Open VSX.

## Use

- **Disk space view** (the ring icon in the activity bar): every cache diskdiet knows,
  biggest first, with how much can go. Expand a row for its folders and notes;
  `node_modules` and `target` list the projects that would be removed. Click a folder
  to reveal it.
- **Free Disk Space…** (the bin icon, or the command palette): tick what to free.
  Package caches are ticked by default; `node_modules`, `target` and Docker only when you
  tick them, and then you choose which projects go. A confirmation shows the total
  before anything is removed. **Free…** on a row frees just that source.
- **Set What Counts as Old…**: 7, 30 (default) or 90 days unused, or everything.
- **Disk full in the terminal**: when a command fails with `ENOSPC` / "no space left on
  device", diskdiet offers **Free Disk Space…**. This needs VS Code's terminal shell
  integration, which is on by default for bash, zsh, fish and PowerShell.
- **Status bar**: shows how much can be freed once it passes 10 GB; click it to open the
  view. Background checks run every 6 hours and notify once per session above the
  threshold.

The extension runs `diskdiet scan --json` and `diskdiet clean --json --yes --only …`,
so it follows exactly the CLI's safety rules.

## Settings

| Setting | Default | |
|---|---|---|
| `diskdiet.olderThanDays` | `30` | What counts as old; `0` counts everything |
| `diskdiet.statusBar` | `aboveThreshold` | `always`, `aboveThreshold` or `never` |
| `diskdiet.warnAboveGB` | `10` | Notify once per session above this; `0` turns it off |
| `diskdiet.checkEveryHours` | `6` | Background checks; `0` turns them off |
| `diskdiet.detectDiskFull` | `true` | Offer to free space when a terminal command runs out of disk |
| `diskdiet.trashProjectFolders` | `false` | Move `node_modules` and `target` folders to the Trash instead of deleting them |
| `diskdiet.path` | | Use another diskdiet binary instead of the built-in one |
