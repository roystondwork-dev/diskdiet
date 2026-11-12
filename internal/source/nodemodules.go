package source

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
	"github.com/roystondwork-dev/diskdiet/internal/units"
)

// projectSource is a folder that projects' tools rebuild: node_modules, or the
// target folder of Rust and Maven projects. A project counts as old when
// nothing at its top level has changed for the given age.
type projectSource struct {
	env    Env
	id     string
	name   string
	folder string
	// finder walks the project folders; sources made together share one, so
	// the home folder is walked once per scan.
	finder *projectFinder
}

// isTarget accepts Rust (Cargo.toml) and Maven (pom.xml) projects: only their
// target folder is build output (a "target" folder could be anything).
func isTarget(project string) bool {
	return fsx.Exists(filepath.Join(project, "Cargo.toml")) || fsx.Exists(filepath.Join(project, "pom.xml"))
}

// NewNodeModules finds projects' node_modules folders.
func NewNodeModules(env Env) Source { return newNodeModules(env, newProjectFinder(env)) }

func newNodeModules(env Env, f *projectFinder) Source {
	return &projectSource{env: env, id: "node_modules", name: "node_modules", folder: "node_modules", finder: f}
}

// NewTargets finds the target folders of Rust and Maven projects; cargo build
// and mvn package recreate them.
func NewTargets(env Env) Source { return newTargets(env, newProjectFinder(env)) }

func newTargets(env Env, f *projectFinder) Source {
	return &projectSource{env: env, id: "target", name: "target folders", folder: "target", finder: f}
}

// IsProjects reports whether a source's plan is a list of project folders
// (see Projects and KeepProjects).
func IsProjects(id string) bool { return id == "node_modules" || id == "target" }

func (s *projectSource) ID() string   { return s.id }
func (s *projectSource) Name() string { return s.name }

// skipUnderHome are folders in the home folder that hold no projects but many
// files (or that macOS guards with permission prompts).
var skipUnderHome = map[string][]string{
	"darwin":  {"Library", "Applications", "Pictures", "Music", "Movies"},
	"windows": {"AppData"},
}

// projectFinder walks the project folders once for node_modules and target
// folders, and remembers the result for the scan it was asked for.
type projectFinder struct {
	env  Env
	mu   sync.Mutex
	key  string
	done chan struct{}
	hits map[string][]string // folder name → paths
	// walks counts walks, for tests.
	walks int
}

func newProjectFinder(env Env) *projectFinder { return &projectFinder{env: env} }

// find returns the folders called name under roots. Calls with the same roots
// and scan time share one walk.
func (f *projectFinder) find(ctx context.Context, roots []string, now time.Time, name string) []string {
	key := now.String() + "\x00" + strings.Join(roots, "\x00")
	f.mu.Lock()
	if f.key != key || f.done == nil {
		f.key = key
		f.walks++
		f.done = make(chan struct{})
		done := f.done
		f.mu.Unlock()
		hits := f.walk(ctx, roots)
		f.mu.Lock()
		f.hits = hits
		close(done)
	}
	done := f.done
	f.mu.Unlock()
	<-done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[name]
}

// walk finds node_modules folders and the target folders of Rust and Maven
// projects. It never descends into hidden folders or into what it found.
func (f *projectFinder) walk(ctx context.Context, roots []string) map[string][]string {
	skip := map[string]bool{}
	for _, name := range skipUnderHome[f.env.GOOS] {
		skip[filepath.Join(f.env.Home, name)] = true
	}
	hits := map[string][]string{}
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
			switch {
			case name == "node_modules":
				hits[name] = append(hits[name], p)
				return fs.SkipDir // never look for projects inside dependencies
			case name == "target" && isTarget(filepath.Dir(p)):
				hits[name] = append(hits[name], p)
				return fs.SkipDir
			case hidden(name) || skip[p]:
				return fs.SkipDir
			}
			return nil
		})
	}
	for _, v := range hits {
		sort.Strings(v)
	}
	return hits
}

// lastTouched is when the project last changed: the newest modification time
// of the project folder and its top-level entries, ignoring the build folder.
func lastTouched(project, folder string) time.Time {
	var newest time.Time
	if info, err := os.Lstat(project); err == nil {
		newest = info.ModTime()
	}
	des, err := os.ReadDir(project)
	if err != nil {
		return newest
	}
	for _, de := range des {
		if de.Name() == folder {
			continue
		}
		if info, err := de.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

func (s *projectSource) Scan(ctx context.Context, o Options) *Report {
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
	dirs := s.finder.find(ctx, r.Locations, o.Now, s.folder)
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
			entries[i] = fsx.Entry{Path: d, Usage: u, Modified: lastTouched(filepath.Dir(d), s.folder)}
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
	if shared > 0 && s.id == "node_modules" {
		r.note(units.FormatBytes(shared) + " of these folders is hard-linked from the pnpm store; pruning the pnpm store afterwards frees it")
	}
	r.plan = plan
	return r
}

func (s *projectSource) Clean(ctx context.Context, r *Report, _ Options) *Result {
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

// KeepProjects removes the given node_modules folders from r's plan, so a
// clean leaves them alone, and updates r's prunable totals.
func KeepProjects(r *Report, keep []string) {
	plans, ok := r.plan.([]entryPlan)
	if !ok || len(keep) == 0 {
		return
	}
	skip := make(map[string]bool, len(keep))
	for _, k := range keep {
		skip[filepath.Clean(k)] = true
	}
	var out []entryPlan
	r.PrunableBytes, r.PrunableItems = 0, 0
	for _, p := range plans {
		var entries []fsx.Entry
		for _, e := range p.entries {
			if skip[filepath.Clean(e.Path)] {
				continue
			}
			entries = append(entries, e)
			r.PrunableBytes += e.Usage.Exclusive
			r.PrunableItems++
		}
		if len(entries) > 0 {
			out = append(out, entryPlan{root: p.root, entries: entries})
		}
	}
	r.plan = out
}
