# The desktop app

## Install

Download the file for your system from the
[latest release](https://github.com/roystondwork-dev/diskdiet/releases/latest). Each
release also has `diskdiet-app_checksums.txt` and build provenance you can check
with `gh attestation verify <file> --repo roystondwork-dev/diskdiet`.

### macOS

1. Open `diskdiet-app_<version>_macos_universal.dmg` and drag **diskdiet** to
   **Applications**.
2. Open diskdiet. macOS says it can't check the app for malicious software, because
   the app isn't notarized by Apple yet. Click **Done**.
3. Open **System Settings → Privacy & Security**, scroll down to the message about
   diskdiet, and click **Open Anyway**. You only do this once.

Prefer the terminal? `xattr -dr com.apple.quarantine /Applications/diskdiet.app`
does the same.

The first scan may make macOS ask whether diskdiet may access folders such as
Desktop or Documents. Allow it if your projects live there, so it can find their
`node_modules`.

### Windows

Run `diskdiet-app_<version>_windows_amd64_setup.exe`. It installs diskdiet for your
user only, so it doesn't need administrator rights. Because the installer isn't
code-signed yet, SmartScreen may warn you: click **More info → Run anyway**.

The app needs Microsoft Edge WebView2, which is built into Windows 10 and 11.

### Linux

```sh
chmod +x diskdiet-app_<version>_linux_x86_64.AppImage
./diskdiet-app_<version>_linux_x86_64.AppImage
```

It needs GTK 3 and WebKit2GTK 4.1, which most desktop distributions include.

## Use

The app scans when it opens. You'll see how much space can be freed, then a row for
each cache:

- **Count as old** chooses what's removable: things unused for 7, 30 or 90 days, or
  everything.
- **Tick or untick** a row to include it. Package caches start ticked; Docker,
  `node_modules` and `target` folders wait until you tick them. For `node_modules`, open the list of
  projects and untick any you want to keep.
- **Docker** volumes are kept unless you tick *Also remove unused volumes*.
- Click a folder name to show it in Finder, Explorer or your file manager.

**Free …** shows exactly what will be removed and asks before doing anything. The
app follows the same safety rules as the CLI: it removes only what the scan showed,
never outside a cache folder, and skips a cache while its package manager is
installing.

## The menu bar / tray icon

diskdiet keeps a small icon in the macOS menu bar (or the Windows and Linux
notification area) after you close its window. Every 6 hours it checks your caches
in the background, and its menu shows how much can be freed.

- **Notify me when** sends a notification when more than 5, 10 (the default), 20 or
  50 GB can be freed, or never. It reminds you at most once a week, unless the amount
  grows a lot. On macOS, the amount also appears beside the icon.
- **Count as old after** chooses 7, 30 (the default) or 90 days for the notifications
  and automatic cleans.
- **Clean automatically** (off by default) removes old entries from the npm, Yarn,
  pnpm and Bun caches every day or every week. It never touches `node_modules` or
  Docker; those always wait for you to look. You get a notification saying how much it
  freed.
- **Open at login** starts diskdiet with only the icon when you log in.
- **Quit diskdiet** stops it, including the background checks.

Settings are kept in `settings.json` in your user settings folder
(`~/Library/Application Support/diskdiet` on macOS, `%AppData%\diskdiet` on Windows,
`~/.config/diskdiet` on Linux).

On Linux, the icon needs a desktop with a system tray (KDE, Cinnamon, Xfce; GNOME
with the AppIndicator extension).
