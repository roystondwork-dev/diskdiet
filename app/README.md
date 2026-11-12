# diskdiet desktop app

The desktop app for macOS, Windows and Linux, built with [Wails](https://wails.io) v3
and Svelte. It uses the same engine as the CLI: `internal/appcore` holds the app's
logic and tests, and `service.go` is a thin layer the window calls.

## Develop

You need Go, Node.js 22+ and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
wails3 doctor        # checks platform dependencies (on Linux: GTK 3 and WebKit2GTK 4.1)
```

Then, from this folder:

```sh
wails3 dev           # run with live reload
wails3 build         # build into bin/
wails3 task package  # .app + .dmg on macOS, NSIS installer on Windows, AppImage etc. on Linux
```

`frontend/bindings` is generated from the Go service by `wails3 generate bindings`
(the build does it for you) and is not committed.
