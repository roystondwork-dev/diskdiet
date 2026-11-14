// Package appcore is the desktop app's logic: scanning, cleaning a chosen
// subset, and revealing folders. The Wails app is a thin layer over it, so
// everything here is testable without a window.
package appcore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/procs"
	"github.com/roystondwork-dev/diskdiet/internal/source"
)

// MaxScanAge is how old a scan may be before Clean asks for a fresh one, so the
// app never removes what it showed a long time ago.
const MaxScanAge = 15 * time.Minute

// Deps is what the core needs from the outside world. Tests replace it.
type Deps struct {
	Sources func() ([]source.Source, string, error) // sources and the home folder
	Busy    func(ctx context.Context) (map[string]string, error)
	Now     func() time.Time
	// Reveal shows a path in the file manager.
	Reveal func(path string) error
	// Demo means the sources are made up (DISKDIET_DEMO=1).
	Demo bool
	// CleanLock, when set, is held while cleaning, so cores that share it
	// (the window and the background watcher) never clean at the same time.
	CleanLock *sync.Mutex
}

// SystemDeps uses the real machine.
func SystemDeps() Deps {
	return Deps{
		Sources: func() ([]source.Source, string, error) {
			env, err := source.SystemEnv()
			if err != nil {
				return nil, "", err
			}
			if source.DemoEnabled(env.Getenv) {
				return source.Demo(env.Home), env.Home, nil
			}
			return source.All(env), env.Home, nil
		},
		Busy:   procs.Busy,
		Now:    time.Now,
		Reveal: reveal,
		Demo:   source.DemoEnabled(os.Getenv),
	}
}

// Core holds the latest scan between calls from the window.
type Core struct {
	d Deps

	mu      sync.Mutex
	busy    bool
	srcs    []source.Source
	reports []*source.Report
	opts    source.Options
	home    string
	scanned time.Time
	paths   map[string]bool // paths the window may reveal
}

// New returns a Core.
func New(d Deps) *Core { return &Core{d: d} }

// ErrBusy is returned while a scan or clean is running.
var ErrBusy = errors.New("diskdiet is already scanning or cleaning")

// ScanRequest says what counts as old.
type ScanRequest struct {
	// OlderThanDays: things unchanged for this many days count as old.
	// 0 counts everything.
	OlderThanDays int `json:"olderThanDays"`
	// Volumes counts unused Docker volumes as removable.
	Volumes bool `json:"volumes"`
	// Only limits the scan to these source IDs; empty means all.
	Only []string `json:"only,omitempty"`
	// Projects are the folders searched for node_modules and target folders;
	// empty means the home folder.
	Projects []string `json:"projects,omitempty"`
}

// Location is a folder the window shows.
type Location struct {
	Path    string `json:"path"`
	Display string `json:"display"`
}

// Project is one node_modules folder a clean would remove.
type Project struct {
	Path     string `json:"path"`
	Display  string `json:"display"`
	Bytes    int64  `json:"bytes"`
	IdleDays int    `json:"idleDays"`
}

// SourceView is one cache, as the window shows it.
type SourceView struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Unavailable   bool       `json:"unavailable"`
	Estimate      bool       `json:"estimate"`
	Bytes         int64      `json:"bytes"`
	PrunableBytes int64      `json:"prunableBytes"`
	Items         int        `json:"items"`
	PrunableItems int        `json:"prunableItems"`
	Locations     []Location `json:"locations"`
	Notes         []string   `json:"notes"`
	Error         string     `json:"error"`
	Projects      []Project  `json:"projects"`
}

// Summary is the result of a scan.
type Summary struct {
	Sources       []SourceView `json:"sources"`
	Missing       []string     `json:"missing"`
	Bytes         int64        `json:"bytes"`
	PrunableBytes int64        `json:"prunableBytes"`
	OlderThanDays int          `json:"olderThanDays"`
	Volumes       bool         `json:"volumes"`
	ScannedAt     time.Time    `json:"scannedAt"`
	// Demo means the data is made up and cleaning removes nothing.
	Demo bool `json:"demo"`
}

// Progress reports one source starting or finishing.
type Progress struct {
	Phase string `json:"phase"` // "scan" or "clean"
	ID    string `json:"id"`
	Name  string `json:"name"`
	Done  bool   `json:"done"`
}

func (c *Core) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.busy {
		return ErrBusy
	}
	c.busy = true
	return nil
}

func (c *Core) finish() {
	c.mu.Lock()
	c.busy = false
	c.mu.Unlock()
}

// Scan measures every source. progress may be nil.
func (c *Core) Scan(ctx context.Context, req ScanRequest, progress func(Progress)) (*Summary, error) {
	if err := c.start(); err != nil {
		return nil, err
	}
	defer c.finish()
	if req.OlderThanDays < 0 {
		return nil, fmt.Errorf("olderThanDays must not be negative, got %d", req.OlderThanDays)
	}
	all, home, err := c.d.Sources()
	if err != nil {
		return nil, err
	}
	srcs := all
	if len(req.Only) > 0 {
		srcs = nil
		for _, s := range all {
			if slices.Contains(req.Only, s.ID()) {
				srcs = append(srcs, s)
			}
		}
	}
	now := c.d.Now()
	o := source.Options{Now: now, Age: time.Duration(req.OlderThanDays) * 24 * time.Hour, Volumes: req.Volumes, Projects: req.Projects}

	report := progressFunc(progress)
	reports := make([]*source.Report, len(srcs))
	var wg sync.WaitGroup
	for i, s := range srcs {
		wg.Go(func() {
			report(Progress{Phase: "scan", ID: s.ID(), Name: s.Name()})
			reports[i] = s.Scan(ctx, o)
			report(Progress{Phase: "scan", ID: s.ID(), Name: s.Name(), Done: true})
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("scan stopped: %w", err)
	}

	sum := &Summary{OlderThanDays: req.OlderThanDays, Volumes: req.Volumes, ScannedAt: now, Missing: []string{}, Sources: []SourceView{}, Demo: c.d.Demo}
	paths := map[string]bool{}
	for _, r := range reports {
		if !r.Found {
			sum.Missing = append(sum.Missing, r.Name)
			continue
		}
		v := view(r, home, now)
		for _, l := range v.Locations {
			paths[l.Path] = true
		}
		for _, p := range v.Projects {
			paths[p.Path] = true
		}
		sum.Sources = append(sum.Sources, v)
		sum.Bytes += r.Bytes
		sum.PrunableBytes += r.PrunableBytes
	}
	sort.SliceStable(sum.Sources, func(i, j int) bool { return sum.Sources[i].Bytes > sum.Sources[j].Bytes })

	c.mu.Lock()
	c.srcs, c.reports, c.opts, c.home, c.scanned, c.paths = srcs, reports, o, home, now, paths
	c.mu.Unlock()
	return sum, nil
}

func progressFunc(p func(Progress)) func(Progress) {
	if p == nil {
		return func(Progress) {}
	}
	var mu sync.Mutex
	return func(pr Progress) {
		mu.Lock()
		defer mu.Unlock()
		p(pr)
	}
}

func view(r *source.Report, home string, now time.Time) SourceView {
	v := SourceView{
		ID: r.ID, Name: r.Name, Unavailable: r.Unavailable, Estimate: r.Estimate,
		Bytes: r.Bytes, PrunableBytes: r.PrunableBytes, Items: r.Items, PrunableItems: r.PrunableItems,
		Notes: append([]string{}, r.Notes...), Error: r.Error,
		Locations: []Location{}, Projects: []Project{},
	}
	for _, l := range r.Locations {
		if filepath.IsAbs(l) {
			v.Locations = append(v.Locations, Location{Path: l, Display: Tildify(l, home)})
		} else {
			v.Locations = append(v.Locations, Location{Display: l}) // e.g. "docker (Docker Desktop)"
		}
	}
	if source.IsProjects(r.ID) {
		for _, p := range source.Projects(r, now) {
			v.Projects = append(v.Projects, Project{
				Path: p.Path, Display: Tildify(filepath.Dir(p.Path), home),
				Bytes: p.Bytes, IdleDays: int(p.Idle.Hours() / 24),
			})
		}
	}
	return v
}

// CleanRequest says which sources to clean.
type CleanRequest struct {
	IDs []string `json:"ids"`
	// KeepProjects are node_modules folders to leave alone.
	KeepProjects []string `json:"keepProjects"`
	// Force cleans even while a package manager is installing.
	Force bool `json:"force"`
	// Trash moves node_modules and target folders to the Trash instead of
	// deleting them.
	Trash bool `json:"trash"`
}

// ResultView is what a clean did for one source.
type ResultView struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Freed   int64    `json:"freed"`
	Trashed int64    `json:"trashed"`
	Removed int      `json:"removed"`
	Skipped string   `json:"skipped"`
	Notes   []string `json:"notes"`
	Errors  []string `json:"errors"`
}

// CleanSummary is the result of a clean.
type CleanSummary struct {
	Results []ResultView `json:"results"`
	Freed   int64        `json:"freed"`
	// Trashed was moved to the Trash: freed once the Trash is emptied.
	Trashed int64 `json:"trashed"`
}

// ErrNoScan is returned when Clean is called without a recent scan.
var ErrNoScan = errors.New("scan again first: the last scan is missing or out of date")

// Clean removes what the last scan planned for the chosen sources. The scan is
// used once; scan again before the next clean.
func (c *Core) Clean(ctx context.Context, req CleanRequest, progress func(Progress)) (*CleanSummary, error) {
	if err := c.start(); err != nil {
		return nil, err
	}
	defer c.finish()
	c.mu.Lock()
	srcs, reports, o, scanned := c.srcs, c.reports, c.opts, c.scanned
	c.srcs, c.reports = nil, nil // a scan is used for one clean only
	c.mu.Unlock()
	if reports == nil || c.d.Now().Sub(scanned) > MaxScanAge {
		return nil, ErrNoScan
	}
	if c.d.CleanLock != nil {
		c.d.CleanLock.Lock()
		defer c.d.CleanLock.Unlock()
	}
	busy := map[string]string{}
	if !req.Force {
		if b, err := c.d.Busy(ctx); err == nil {
			busy = b
		}
	}
	report := progressFunc(progress)
	sum := &CleanSummary{Results: []ResultView{}}
	for i, s := range srcs {
		r := reports[i]
		if !slices.Contains(req.IDs, s.ID()) || (r.PrunableBytes == 0 && r.PrunableItems == 0) {
			continue
		}
		if source.IsProjects(s.ID()) {
			source.KeepProjects(r, req.KeepProjects)
			if r.PrunableItems == 0 {
				continue
			}
		}
		if what, ok := busy[s.ID()]; ok {
			sum.Results = append(sum.Results, ResultView{ID: s.ID(), Name: s.Name(), Notes: []string{}, Errors: []string{},
				Skipped: fmt.Sprintf("%q is running; try again when it finishes", what)})
			continue
		}
		report(Progress{Phase: "clean", ID: s.ID(), Name: s.Name()})
		co := o
		co.Trash = req.Trash
		res := s.Clean(ctx, r, co)
		report(Progress{Phase: "clean", ID: s.ID(), Name: s.Name(), Done: true})
		sum.Results = append(sum.Results, ResultView{
			ID: res.ID, Name: res.Name, Freed: res.Freed, Trashed: res.Trashed, Removed: res.Removed, Skipped: res.Skipped,
			Notes: append([]string{}, res.Notes...), Errors: append([]string{}, res.Errors...),
		})
		sum.Freed += res.Freed
		sum.Trashed += res.Trashed
		if ctx.Err() != nil {
			break
		}
	}
	return sum, nil
}

// Reveal shows a folder from the last scan in the file manager. Other paths
// are refused, so the window can't be used to open arbitrary files.
func (c *Core) Reveal(path string) error {
	c.mu.Lock()
	ok := c.paths[path]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s is not a folder from the last scan", path)
	}
	return c.d.Reveal(path)
}

// Tildify shortens paths in the home folder to ~/….
func Tildify(p, home string) string {
	if home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return "~" + string(filepath.Separator) + rel
	}
	return p
}

// revealCommand is the file-manager command for an OS.
func revealCommand(goos, path string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{"-R", path}
	case "windows":
		return "explorer", []string{"/select," + path}
	default:
		return "xdg-open", []string{filepath.Dir(path)}
	}
}

func reveal(path string) error {
	name, args := revealCommand(runtime.GOOS, path)
	cmd := exec.Command(name, args...) //nolint:gosec,noctx // G204: fixed file-manager command; it outlives the call
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
