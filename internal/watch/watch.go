// Package watch is the tray's background job: it scans now and then, warns
// when a lot of space could be freed, and, only if the user turns it on,
// cleans old package-cache entries on a schedule.
//
// It has no GUI code; the app shows its Status in the tray menu and passes its
// notifications on to the OS.
package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/appcore"
	"github.com/roystondwork-dev/diskdiet/internal/units"
)

// Auto-clean schedules.
const (
	AutoOff    = "off"
	AutoDaily  = "daily"
	AutoWeekly = "weekly"
)

// SafeAutoSources are the only caches an automatic clean touches: removing
// their old entries costs at most a re-download. node_modules and Docker are
// never cleaned without the user looking.
var SafeAutoSources = []string{"npm", "yarn", "pnpm", "bun"}

// Settings are the tray's choices and the little state it keeps between runs.
type Settings struct {
	// CheckEveryHours is how often to scan in the background.
	CheckEveryHours int `json:"checkEveryHours"`
	// WarnAboveGB: notify when at least this much can be freed. 0 is off.
	WarnAboveGB float64 `json:"warnAboveGB"`
	// OlderThanDays: what counts as old, for warnings and automatic cleans.
	OlderThanDays int `json:"olderThanDays"`
	// AutoClean is AutoOff, AutoDaily or AutoWeekly.
	AutoClean string `json:"autoClean"`

	LastCheck         time.Time `json:"lastCheck"`
	LastNotified      time.Time `json:"lastNotified"`
	LastNotifiedBytes int64     `json:"lastNotifiedBytes"`
	LastAutoClean     time.Time `json:"lastAutoClean"`
}

// Defaults are the settings on first run: check every 6 hours, warn above
// 10 GB of things unused for 30 days, never clean on its own.
func Defaults() Settings {
	return Settings{CheckEveryHours: 6, WarnAboveGB: 10, OlderThanDays: 30, AutoClean: AutoOff}
}

// normalize replaces missing or invalid values with defaults.
func (s Settings) normalize() Settings {
	d := Defaults()
	if s.CheckEveryHours < 1 || s.CheckEveryHours > 24*7 {
		s.CheckEveryHours = d.CheckEveryHours
	}
	if s.WarnAboveGB < 0 {
		s.WarnAboveGB = d.WarnAboveGB
	}
	if s.OlderThanDays < 1 {
		s.OlderThanDays = d.OlderThanDays
	}
	switch s.AutoClean {
	case AutoOff, AutoDaily, AutoWeekly:
	default:
		s.AutoClean = AutoOff
	}
	return s
}

// Store keeps Settings in a JSON file.
type Store struct{ path string }

// NewStore stores settings at path.
func NewStore(path string) *Store { return &Store{path: path} }

// DefaultPath is diskdiet's settings file in the user's config folder.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding your settings folder: %w", err)
	}
	return filepath.Join(dir, "diskdiet", "settings.json"), nil
}

// Load reads the settings; a missing file gives the defaults.
func (s *Store) Load() (Settings, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Defaults(), fmt.Errorf("reading settings: %w", err)
	}
	set := Defaults()
	if err := json.Unmarshal(data, &set); err != nil {
		return Defaults(), fmt.Errorf("reading settings %s: %w", s.path, err)
	}
	return set.normalize(), nil
}

// Save writes the settings atomically.
func (s *Store) Save(set Settings) error {
	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".settings-*")
	if err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	_, err = tmp.Write(append(data, '\n'))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), s.path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("saving settings: %w", err)
	}
	return nil
}

// ShouldNotify decides whether a check that found prunable bytes deserves a
// notification: above the threshold, and either never told, told over a week
// ago, or grown by half the threshold since.
func ShouldNotify(s Settings, prunable int64, now time.Time) bool {
	threshold := int64(s.WarnAboveGB * 1e9)
	if threshold <= 0 || prunable < threshold {
		return false
	}
	if s.LastNotified.IsZero() || now.Sub(s.LastNotified) >= 7*24*time.Hour {
		return true
	}
	return prunable >= s.LastNotifiedBytes+threshold/2
}

// AutoCleanDue reports whether an automatic clean should run now.
func AutoCleanDue(s Settings, now time.Time) bool {
	var every time.Duration
	switch s.AutoClean {
	case AutoDaily:
		every = 24 * time.Hour
	case AutoWeekly:
		every = 7 * 24 * time.Hour
	default:
		return false
	}
	return s.LastAutoClean.IsZero() || now.Sub(s.LastAutoClean) >= every
}

// Status is what the tray shows.
type Status struct {
	Checking      bool      `json:"checking"`
	Checked       time.Time `json:"checked"`
	Bytes         int64     `json:"bytes"`
	PrunableBytes int64     `json:"prunableBytes"`
	Err           string    `json:"err"`
	// LastFreed is what the latest automatic clean freed.
	LastFreed int64 `json:"lastFreed"`
}

// Notifier shows a desktop notification.
type Notifier func(title, body string) error

// Deps is what a Watcher needs. Tests replace it.
type Deps struct {
	Core     *appcore.Core
	Store    *Store
	Notify   Notifier
	Now      func() time.Time
	OnStatus func(Status)
	// StartDelay is how long after start the first check waits, so it
	// doesn't slow down logging in.
	StartDelay time.Duration
}

// Watcher runs the background checks.
type Watcher struct {
	d    Deps
	wake chan struct{}

	mu       sync.Mutex
	settings Settings
	status   Status
}

// New loads settings and returns a Watcher. A broken settings file is
// replaced by defaults.
func New(d Deps) *Watcher {
	set, _ := d.Store.Load()
	return &Watcher{d: d, settings: set, wake: make(chan struct{}, 1)}
}

// Settings returns the current settings.
func (w *Watcher) Settings() Settings {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.settings
}

// Status returns the latest status.
func (w *Watcher) Status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}

// Update changes the settings, saves them and checks again.
func (w *Watcher) Update(change func(*Settings)) error {
	w.mu.Lock()
	set := w.settings
	change(&set)
	set = set.normalize()
	w.settings = set
	w.mu.Unlock()
	err := w.d.Store.Save(set)
	w.CheckNow()
	return err
}

// CheckNow asks Run to check as soon as it can.
func (w *Watcher) CheckNow() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *Watcher) setStatus(change func(*Status)) {
	w.mu.Lock()
	change(&w.status)
	st := w.status
	w.mu.Unlock()
	if w.d.OnStatus != nil {
		w.d.OnStatus(st)
	}
}

// Run checks after StartDelay, then whenever a check is due or asked for,
// until ctx ends.
func (w *Watcher) Run(ctx context.Context) {
	timer := time.NewTimer(w.d.StartDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-w.wake:
		}
		w.Check(ctx)
		set := w.Settings()
		timer.Reset(time.Duration(set.CheckEveryHours) * time.Hour)
	}
}

// Check scans once, notifies if warranted and cleans automatically if due.
func (w *Watcher) Check(ctx context.Context) {
	set := w.Settings()
	w.setStatus(func(s *Status) { s.Checking = true; s.Err = "" })
	sum, err := w.d.Core.Scan(ctx, appcore.ScanRequest{OlderThanDays: set.OlderThanDays}, nil)
	now := w.d.Now()
	if err != nil {
		w.setStatus(func(s *Status) { s.Checking = false; s.Err = err.Error() })
		return
	}
	w.setStatus(func(s *Status) {
		s.Checking, s.Checked, s.Bytes, s.PrunableBytes = false, now, sum.Bytes, sum.PrunableBytes
	})
	set = w.Settings() // may have changed during the scan
	set.LastCheck = now

	if AutoCleanDue(set, now) {
		freed, cleaned := w.autoClean(ctx, sum)
		set.LastAutoClean = now
		if cleaned {
			w.setStatus(func(s *Status) { s.LastFreed = freed; s.PrunableBytes -= min(freed, s.PrunableBytes) })
			if freed > 0 {
				w.notify("diskdiet freed "+units.FormatBytes(freed),
					fmt.Sprintf("Removed npm, Yarn, pnpm and Bun cache entries unused for %d days. You can turn this off in the menu.", set.OlderThanDays))
			}
		}
	}
	if left := w.Status().PrunableBytes; ShouldNotify(set, left, now) {
		if w.notify(units.FormatBytes(left)+" of developer caches can be freed",
			fmt.Sprintf("Unused for %d days. Open diskdiet to see what and free it.", set.OlderThanDays)) {
			set.LastNotified, set.LastNotifiedBytes = now, left
		}
	}

	w.mu.Lock()
	cur := w.settings
	cur.LastCheck, cur.LastNotified, cur.LastNotifiedBytes, cur.LastAutoClean =
		set.LastCheck, set.LastNotified, set.LastNotifiedBytes, set.LastAutoClean
	w.settings = cur
	w.mu.Unlock()
	_ = w.d.Store.Save(cur)
}

// autoClean cleans the safe sources the scan found something old in.
func (w *Watcher) autoClean(ctx context.Context, sum *appcore.Summary) (freed int64, cleaned bool) {
	var ids []string
	for _, s := range sum.Sources {
		if slices.Contains(SafeAutoSources, s.ID) && (s.PrunableBytes > 0 || s.PrunableItems > 0) {
			ids = append(ids, s.ID)
		}
	}
	if len(ids) == 0 {
		return 0, true
	}
	res, err := w.d.Core.Clean(ctx, appcore.CleanRequest{IDs: ids}, nil)
	if err != nil {
		return 0, false
	}
	return res.Freed, true
}

func (w *Watcher) notify(title, body string) bool {
	if w.d.Notify == nil {
		return false
	}
	return w.d.Notify(title, body) == nil
}

// Describe is the tray's one-line summary of a status.
func Describe(st Status) string {
	switch {
	case st.Checking:
		return "Checking…"
	case st.Err != "":
		return "Couldn't check: " + firstLine(st.Err)
	case st.Checked.IsZero():
		return "Not checked yet"
	case st.PrunableBytes == 0:
		return "Nothing old to free"
	default:
		return units.FormatBytes(st.PrunableBytes) + " can be freed"
	}
}

// Detail is the tray's second line: when it last checked and the total.
func Detail(st Status, set Settings, now time.Time) string {
	if st.Checked.IsZero() {
		return fmt.Sprintf("Checks every %d hours", set.CheckEveryHours)
	}
	when := st.Checked.Format("15:04")
	if now.Sub(st.Checked) >= 24*time.Hour || now.Day() != st.Checked.Day() {
		when = st.Checked.Format("Jan 2, 15:04")
	}
	return fmt.Sprintf("Unused for %d days · of %s · checked %s", set.OlderThanDays, units.FormatBytes(st.Bytes), when)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 60 {
		s = s[:57] + "…"
	}
	return s
}
