// Command diskdiet-app is diskdiet's desktop app: a window that shows where
// developer disk space went and frees what you choose.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/roystondwork-dev/diskdiet/internal/appcore"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[appcore.Progress]("progress")
}

func main() {
	svc := &DiskService{core: appcore.New(appcore.SystemDeps())}
	app := application.New(application.Options{
		Name:        "diskdiet",
		Description: "See where your developer disk space went, and get it back safely",
		Services:    []application.Service{application.NewService(svc)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	svc.app = app

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "diskdiet",
		Width:     980,
		Height:    720,
		MinWidth:  720,
		MinHeight: 520,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 40,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(252, 252, 251),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
