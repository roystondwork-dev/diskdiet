package main

import (
	_ "embed"
	"fmt"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/roystondwork-dev/diskdiet/internal/units"
	"github.com/roystondwork-dev/diskdiet/internal/watch"
)

//go:embed icons/tray-template.png
var trayTemplateIcon []byte

//go:embed icons/tray.png
var trayIcon []byte

// trayUI is the menu-bar (macOS) or notification-area (Windows, Linux) icon
// and its menu. It shows the watcher's status and changes its settings.
type trayUI struct {
	app     *application.App
	window  *application.WebviewWindow
	watcher *watch.Watcher
	tray    *application.SystemTray
	menu    *application.Menu
	status  *application.MenuItem
	detail  *application.MenuItem
	login   *application.MenuItem
}

var warnChoices = []struct {
	gb    float64
	label string
}{
	{5, "More than 5 GB can be freed"},
	{10, "More than 10 GB can be freed"},
	{20, "More than 20 GB can be freed"},
	{50, "More than 50 GB can be freed"},
	{0, "Never"},
}

var ageChoices = []int{7, 30, 90}

var autoChoices = []struct {
	value, label string
}{
	{watch.AutoOff, "Never"},
	{watch.AutoDaily, "Every day"},
	{watch.AutoWeekly, "Every week"},
}

func newTray(app *application.App, window *application.WebviewWindow, w *watch.Watcher) *trayUI {
	t := &trayUI{app: app, window: window, watcher: w}
	t.tray = app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		t.tray.SetTemplateIcon(trayTemplateIcon)
	} else {
		t.tray.SetIcon(trayIcon)
	}
	t.tray.SetTooltip("diskdiet")
	t.build()
	t.tray.SetMenu(t.menu)
	return t
}

func (t *trayUI) build() {
	set := t.watcher.Settings()
	m := t.app.NewMenu()
	t.status = m.Add(watch.Describe(t.watcher.Status())).SetEnabled(false)
	t.detail = m.Add(watch.Detail(t.watcher.Status(), set, time.Now())).SetEnabled(false)
	m.AddSeparator()
	m.Add("Open diskdiet").OnClick(func(*application.Context) { t.showWindow() })
	m.Add("Check now").OnClick(func(*application.Context) { t.watcher.CheckNow() })
	m.AddSeparator()

	warn := m.AddSubmenu("Notify me when")
	for _, c := range warnChoices {
		gb := c.gb
		warn.AddRadio(c.label, set.WarnAboveGB == gb).OnClick(func(*application.Context) {
			_ = t.watcher.Update(func(s *watch.Settings) { s.WarnAboveGB = gb })
		})
	}

	age := m.AddSubmenu("Count as old after")
	for _, days := range ageChoices {
		d := days
		age.AddRadio(fmt.Sprintf("%d days unused", d), set.OlderThanDays == d).OnClick(func(*application.Context) {
			_ = t.watcher.Update(func(s *watch.Settings) { s.OlderThanDays = d })
		})
	}

	auto := m.AddSubmenu("Clean automatically")
	for _, c := range autoChoices {
		v := c.value
		auto.AddRadio(c.label, set.AutoClean == v).OnClick(func(*application.Context) {
			_ = t.watcher.Update(func(s *watch.Settings) { s.AutoClean = v })
		})
	}
	auto.AddSeparator()
	auto.Add("Only old npm, Yarn, pnpm and Bun cache entries.").SetEnabled(false)
	auto.Add("Never node_modules or Docker.").SetEnabled(false)

	t.login = m.AddCheckbox("Open at login", false).OnClick(func(ctx *application.Context) {
		t.setLogin(ctx.ClickedMenuItem().Checked())
	})
	m.AddSeparator()
	m.Add("Quit diskdiet").OnClick(func(*application.Context) { t.app.Quit() })
	t.menu = m
}

// started runs once the app is up: autostart status needs a running app.
func (t *trayUI) started() {
	if on, err := t.app.Autostart.IsEnabled(); err == nil {
		t.login.SetChecked(on)
		t.menu.Update()
	}
}

func (t *trayUI) setLogin(on bool) {
	var err error
	if on {
		err = t.app.Autostart.EnableWithOptions(application.AutostartOptions{Arguments: []string{backgroundFlag}})
	} else {
		err = t.app.Autostart.Disable()
	}
	if err != nil {
		t.login.SetChecked(!on)
		t.menu.Update()
		t.app.Dialog.Error().SetTitle("diskdiet").SetMessage("Couldn't change Open at login: " + err.Error()).Show()
	}
}

// refresh shows a new status. It may be called from any goroutine.
func (t *trayUI) refresh(st watch.Status) {
	set := t.watcher.Settings()
	application.InvokeAsync(func() {
		t.status.SetLabel(watch.Describe(st))
		t.detail.SetLabel(watch.Detail(st, set, time.Now()))
		label, tip := "", "diskdiet"
		if !st.Checking && st.Err == "" && st.PrunableBytes > 0 {
			tip = "diskdiet: " + units.FormatBytes(st.PrunableBytes) + " can be freed"
			if set.WarnAboveGB > 0 && st.PrunableBytes >= int64(set.WarnAboveGB*1e9) {
				label = units.FormatBytes(st.PrunableBytes) // macOS shows it beside the icon
			}
		}
		t.tray.SetLabel(label)
		t.tray.SetTooltip(tip)
		t.menu.Update()
	})
}

func (t *trayUI) showWindow() {
	t.window.Show()
	t.window.Focus()
}
