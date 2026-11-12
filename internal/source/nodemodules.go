package source

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
	"github.com/roystondwork-dev/diskdiet/internal/units"
)

// nodeModulesSource is the node_modules folders of projects. A project counts
// as old when nothing at its top level has changed for the given age.
type nodeModulesSource struct{ env Env }

// NewNodeModules finds projects' node_modules folders.
func NewNodeModules(env Env) Source { return &nodeModulesSource{env: env} }

func (s *nodeModulesSource) ID() string   { return "node_modules" }
func (s *nodeModulesSource) Name() string { return "node_modules" }

// skipUnderHome are folders in the home folder that hold no projects but many
// files (or that macOS guards with permission prompts).
var skipUnderHome = map[string][]string{
	"darwin":  {"Library", "Applications", "Pictures", "Music", "Movies"},
	"windows": {"AppData"},
}

// find walks roots for node_modules folders. It never descends into hidden
// folders or into a node_modules it found.
func (s *nodeModulesSource) find(ctx context.Context, roots []string) []string {
	skip := map[string]bool{}
	for _, name := range skipUnderHome[s.env.GOOS] {
		skip[filepath.Join(s.env.Home, name)] = true
	}
	var found []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil || !d.IsDir() {
				if err != nil && d != nil && d.IsDir() && p != root {
					return fs.SkipDir
				}
				return nil
			}
			if p == root {
				return nil
			}
			name := d.Name()
			if name == "node_modules" {
				found = append(found, p)
				return fs.SkipDir
			}
			if hidden(name) || skip[p] {
				return fs.SkipDir
			}
			return nil
		})
	}
	sort.Strings(found)
	return found
}

// lastTouched is when the project last changed: the newest modification time
// of the project folder and its top-level entries, ignoring node_modules.
func lastTouched(project string) time.Time {
	var newest time.Time
	if info, err := os.Lstat(project); err == nil {
		newest = info.ModTime()
	}
	des, err := os.ReadDir(project)
	if err != nil {
		return newest
	}
	for _, de := range des {
		if de.Name() == "node_modules" {
			continue
		}
		if info, err := de.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

func (s *nodeModulesSource) Scan(ctx context.Context, o Options) *Report {
	r := &Report{ID: s.ID(), Name: s.Name()}
	roots := o.Projects
	if len(roots) == 0 {
		roots = []string{s.env.Home}
	}
	for _, root := range roots {
		if fsx.IsDir(root) {
			r.Locations = append(r.Locations, root)
		} else {
			r.note("not a folder: " + root)
		}
	}
	dirs := s.find(ctx, r.Locations)
	entries := make([]fsx.Entry, len(dirs))
	// Hard-link counts are costly on Windows, where pnpm users are rarer.
	opts := fsx.Options{CountLinks: s.env.GOOS != "windows"}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, d := range dirs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			u, _ := fsx.DirSize(ctx, d, opts)
			entries[i] = fsx.Entry{Path: d, Usage: u, Modified: lastTouched(filepath.Dir(d))}
		}()
	}
	wg.Wait()

	var plan []entryPlan
	var shared int64
	for _, e := range entries {
		r.Found = true
		r.Bytes += e.Usage.Bytes
		r.Items++
		if o.Old(e.Modified) {
			plan = append(plan, entryPlan{root: filepath.Dir(e.Path), entries: []fsx.Entry{e}})
			r.PrunableBytes += e.Usage.Exclusive
			r.PrunableItems++
			shared += e.Usage.Bytes - e.Usage.Exclusive
		}
	}
	if shared > 0 {
		r.note(units.FormatBytes(shared) + " of these folders is hard-linked from the pnpm store; pruning the pnpm store afterwards frees it")
	}
	r.plan = plan
	return r
}

func (s *nodeModulesSource) Clean(ctx context.Context, r *Report, _ Options) *Result {
	res := &Result{ID: s.ID(), Name: s.Name()}
	plans, _ := r.plan.([]entryPlan)
	removeEntries(ctx, plans, res)
	return res
}

// Projects lists the planned node_modules folders with their project's age,
// for previews.
func Projects(r *Report, now time.Time) []ProjectInfo {
	plans, _ := r.plan.([]entryPlan)
	out := make([]ProjectInfo, 0, len(plans))
	for _, p := range plans {
		for _, e := range p.entries {
			out = append(out, ProjectInfo{Path: e.Path, Bytes: e.Usage.Exclusive, Idle: now.Sub(e.Modified)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out
}

// ProjectInfo describes one node_modules folder a clean would remove.
type ProjectInfo struct {
	Path  string        `json:"path"`
	Bytes int64         `json:"bytes"`
	Idle  time.Duration `json:"idleNanoseconds"`
}
