// Command diskdiet-app is diskdiet's desktop app: a window that shows where
// developer disk space went and frees what you choose, and a tray icon that
// checks in the background and warns when a lot can be freed.
package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/roystondwork-dev/diskdiet/internal/appcore"
	"github.com/roystondwork-dev/diskdiet/internal/watch"
)

//go:embed all:frontend/dist
var assets embed.FS

// backgroundFlag starts the app with only the tray icon (used at login).
const backgroundFlag = "--background"

func init() {
	application.RegisterEvent[appcore.Progress]("progress")
}

func main() {
	background := slices.Contains(os.Args[1:], backgroundFlag)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The window and the background watcher have separate scans, but never
	// clean at the same time.
	cleanLock := &sync.Mutex{}
	deps := appcore.SystemDeps()
	deps.CleanLock = cleanLock
	svc := &DiskService{core: appcore.New(deps)}
	notifier := notifications.New()

	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "diskdiet",
		Description: "See where your developer disk space went, and get it back safely",
		Services: []application.Service{
			application.NewService(svc),
			application.NewService(notifier),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		// Closing the window keeps the tray running; quit from the tray menu.
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "io.github.roystondwork-dev.diskdiet",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window != nil {
					application.InvokeAsync(func() { window.Show(); window.Focus() })
				}
			},
		},
		OnShutdown: cancel,
	})
	svc.app = app

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "diskdiet",
		Width:     980,
		Height:    720,
		MinWidth:  720,
		MinHeight: 520,
		Hidden:    background,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 40,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(252, 252, 251),
		URL:              "/",
	})
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})
	// Clicking the Dock icon brings the window back.
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) {
		window.Show()
		window.Focus()
	})

	store := watch.NewStore(settingsPath())
	var tray *trayUI
	watcher := watch.New(watch.Deps{
		Core:       appcore.New(deps),
		Store:      store,
		Notify:     notifyFunc(notifier),
		Now:        time.Now,
		StartDelay: 20 * time.Second,
		OnStatus: func(st watch.Status) {
			if tray != nil {
				tray.refresh(st)
			}
		},
	})
	tray = newTray(app, window, watcher)
	notifier.OnNotificationResponse(func(notifications.NotificationResult) {
		application.InvokeAsync(tray.showWindow)
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		tray.started()
		go watcher.Run(ctx)
	})

	err := app.Run()
	cancel()
	if err != nil {
		log.Fatal(err) //nolint:gocritic // cancel has run
	}
}

func settingsPath() string {
	if p, err := watch.DefaultPath(); err == nil {
		return p
	}
	return "diskdiet-settings.json"
}

// notifyFunc sends desktop notifications, asking for permission the first time.
func notifyFunc(n *notifications.NotificationService) watch.Notifier {
	return func(title, body string) error {
		ok, err := n.CheckNotificationAuthorization()
		if err != nil || !ok {
			ok, err = n.RequestNotificationAuthorization()
		}
		if err != nil {
			return err //nolint:wrapcheck // shown to nobody; the watcher only retries
		}
		if !ok {
			return errors.New("notifications are turned off for diskdiet")
		}
		return n.SendNotification(notifications.NotificationOptions{ //nolint:wrapcheck // as above
			ID:    "diskdiet-" + strconv.FormatInt(time.Now().UnixNano(), 10),
			Title: title,
			Body:  body,
		})
	}
}
