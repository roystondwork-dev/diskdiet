module github.com/roystondwork-dev/diskdiet/app

go 1.26.0

toolchain go1.27.4

require (
	github.com/roystondwork-dev/diskdiet v0.0.0
	github.com/wailsapp/wails/v3 v3.1.0
)

require (
	github.com/coder/websocket v1.8.14 // indirect
	github.com/ebitengine/purego v0.11.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/power-devops/perfstat v0.0.0-20260805114148-88456608a4f6 // indirect
	github.com/shirou/gopsutil/v4 v4.26.10 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.48.0 // indirect
)

// The app is built from the same commit as the CLI.
replace github.com/roystondwork-dev/diskdiet => ../
