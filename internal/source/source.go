// Package source finds the caches developers' tools leave on disk, measures
// them, and removes what the user chooses.
//
// Each Source scans first. The scan records exactly what a clean would remove
// (its plan), so Clean removes what the user previewed and nothing else.
package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/runner"
)

// Env is what sources need from the machine. Tests replace its parts.
type Env struct {
	Home   string
	GOOS   string
	Getenv func(string) string
	Run    runner.Runner
}

// SystemEnv describes the running machine.
func SystemEnv() (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, fmt.Errorf("finding your home folder: %w", err)
	}
	return Env{Home: home, GOOS: runtime.GOOS, Getenv: os.Getenv, Run: runner.System{}}, nil
}

// localAppData is %LOCALAPPDATA% on Windows.
func (e Env) localAppData() string {
	if v := e.Getenv("LOCALAPPDATA"); v != "" {
		return v
	}
	return filepath.Join(e.Home, "AppData", "Local")
}

// cacheHome is the per-user cache folder: ~/Library/Caches, $XDG_CACHE_HOME
// (~/.cache) or %LOCALAPPDATA%.
func (e Env) cacheHome() string {
	switch e.GOOS {
	case "darwin":
		return filepath.Join(e.Home, "Library", "Caches")
	case "windows":
		return e.localAppData()
	default:
		if v := e.Getenv("XDG_CACHE_HOME"); v != "" {
			return v
		}
		return filepath.Join(e.Home, ".cache")
	}
}

// dataHome is the per-user data folder: ~/Library, $XDG_DATA_HOME
// (~/.local/share) or %LOCALAPPDATA%.
func (e Env) dataHome() string {
	switch e.GOOS {
	case "darwin":
		return filepath.Join(e.Home, "Library")
	case "windows":
		return e.localAppData()
	default:
		if v := e.Getenv("XDG_DATA_HOME"); v != "" {
			return v
		}
		return filepath.Join(e.Home, ".local", "share")
	}
}

// Options say what counts as old enough to remove.
type Options struct {
	// Age: entries last changed longer ago than this can be removed.
	// Zero means everything can be removed.
	Age time.Duration
	// Now is the reference time for Age.
	Now time.Time
	// Volumes allows removing unused Docker volumes, which can hold data.
	Volumes bool
	// Projects are the folders searched for node_modules. Empty means the
	// home folder.
	Projects []string
	// Trash moves node_modules and target folders to the Trash instead of
	// deleting them, so they can be put back. Caches are always deleted.
	Trash bool
}

// Old reports whether something last changed at t is old enough to remove.
func (o Options) Old(t time.Time) bool {
	return o.Age <= 0 || t.Before(o.Now.Add(-o.Age))
}

// Report is what a scan found for one source.
type Report struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Found     bool     `json:"found"`
	Locations []string `json:"locations,omitempty"`
	// Bytes is the space the source takes on disk.
	Bytes int64 `json:"bytes"`
	// Items counts what the source holds (packages, files, folders, images).
	Items int `json:"items"`
	// PrunableBytes is the space a clean would free.
	PrunableBytes int64 `json:"prunableBytes"`
	// PrunableItems counts what a clean would remove.
	PrunableItems int `json:"prunableItems"`
	// Estimate marks PrunableBytes as an estimate: the tool that owns the
	// cache (Docker, pnpm) decides what to remove.
	Estimate bool `json:"estimate,omitempty"`
	// Unavailable means the source exists but could not be measured, such
	// as Docker installed but not running.
	Unavailable bool     `json:"unavailable,omitempty"`
	Notes       []string `json:"notes,omitempty"`
	Error       string   `json:"error,omitempty"`

	plan any
}

func (r *Report) note(s string) { r.Notes = append(r.Notes, s) }

// Result is what a clean did for one source.
type Result struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Freed int64  `json:"freedBytes"`
	// Trashed is what was moved to the Trash: freed only once it's emptied.
	Trashed int64    `json:"trashedBytes,omitempty"`
	Removed int      `json:"removed"`
	Skipped string   `json:"skipped,omitempty"`
	Notes   []string `json:"notes,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

func (r *Result) fail(err error) {
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
}

// Source is one kind of cache.
type Source interface {
	ID() string
	Name() string
	// Scan measures the source and plans what a clean would remove.
	Scan(ctx context.Context, o Options) *Report
	// Clean removes what Scan planned in r.
	Clean(ctx context.Context, r *Report, o Options) *Result
}

// All returns every source, in display order.
func All(env Env) []Source {
	projects := newProjectFinder(env) // one walk for node_modules and target folders
	return []Source{
		NewNPM(env),
		NewYarn(env),
		NewPnpm(env),
		NewBun(env),
		NewPip(env),
		NewUV(env),
		NewCargo(env),
		NewGo(env),
		NewGradle(env),
		NewMaven(env),
		NewPlaywright(env),
		NewCypress(env),
		NewXcode(env),
		NewDocker(env),
		newNodeModules(env, projects),
		newTargets(env, projects),
	}
}

// IDs lists the IDs of all sources.
func IDs() []string {
	return []string{"npm", "yarn", "pnpm", "bun", "pip", "uv", "cargo", "go", "gradle", "maven",
		"playwright", "cypress", "xcode", "docker", "node_modules", "target"}
}
