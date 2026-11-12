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
- **Tick or untick** a row to include it. For `node_modules`, open the list of
  projects and untick any you want to keep.
- **Docker** volumes are kept unless you tick *Also remove unused volumes*.
- Click a folder name to show it in Finder, Explorer or your file manager.

**Free …** shows exactly what will be removed and asks before doing anything. The
app follows the same safety rules as the CLI: it removes only what the scan showed,
never outside a cache folder, and skips a cache while its package manager is
installing.
