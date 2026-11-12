package source

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

// location is a folder whose contents are removable entries.
type location struct {
	root string
	// files makes every file under root an entry; otherwise each direct
	// child (a package folder or archive) is one entry.
	files bool
	skip  func(name string) bool
	// find, when set, walks root and makes each folder it accepts an entry,
	// without looking inside it (e.g. Maven's version folders).
	find func(path, name string) bool
	// keep names entries that stay whatever their age, such as the newest
	// version of each browser.
	keep func(entries []fsx.Entry) map[string]bool
	// show is the folder to report instead of root, when root is one of
	// many folders inside a cache.
	show string
}

// entryPlan lists the entries to remove under one root.
type entryPlan struct {
	root    string
	entries []fsx.Entry
}

// scanLocations measures each existing location into r and returns what is old
// enough to remove.
func scanLocations(ctx context.Context, r *Report, locs []location, o Options) []entryPlan {
	var plans []entryPlan
	for _, loc := range locs {
		if !fsx.IsDir(loc.root) {
			continue
		}
		var entries []fsx.Entry
		var err error
		switch {
		case loc.find != nil:
			entries, err = fsx.Find(ctx, loc.root, fsx.Options{}, loc.find)
		case loc.files:
			entries, err = fsx.Files(ctx, loc.root, fsx.Options{})
		default:
			entries, err = fsx.Children(ctx, loc.root, fsx.Options{}, loc.skip)
		}
		if err != nil && ctx.Err() == nil {
			r.note("could not read " + loc.root + ": " + err.Error())
		}
		r.Found = true
		show := loc.root
		if loc.show != "" {
			show = loc.show
		}
		r.Locations = appendUnique(r.Locations, show)
		var keep map[string]bool
		if loc.keep != nil {
			keep = loc.keep(entries)
		}
		p := entryPlan{root: loc.root}
		for _, e := range entries {
			r.Bytes += e.Usage.Bytes
			r.Items++
			if !keep[e.Path] && o.Old(e.Modified) {
				p.entries = append(p.entries, e)
				r.PrunableBytes += e.Usage.Exclusive
				r.PrunableItems++
			}
		}
		if len(p.entries) > 0 {
			plans = append(plans, p)
		}
	}
	return plans
}

// removeEntries deletes planned entries, adding what was freed to res.
func removeEntries(ctx context.Context, plans []entryPlan, res *Result) {
	for _, p := range plans {
		for _, e := range p.entries {
			if ctx.Err() != nil {
				res.fail(ctx.Err())
				return
			}
			if err := fsx.Remove(p.root, e.Path); err != nil {
				res.fail(err)
				continue
			}
			res.Freed += e.Usage.Exclusive
			res.Removed++
		}
	}
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func hidden(name string) bool { return strings.HasPrefix(name, ".") }

// dirSource is a cache made of package folders or archives, each removable on
// its own: Yarn's and Bun's caches.
type dirSource struct {
	id, name string
	locate   func(ctx context.Context) []location
}

func (s *dirSource) ID() string   { return s.id }
func (s *dirSource) Name() string { return s.name }

func (s *dirSource) Scan(ctx context.Context, o Options) *Report {
	r := &Report{ID: s.id, Name: s.name}
	r.plan = scanLocations(ctx, r, s.locate(ctx), o)
	if r.Items == 0 && len(r.Notes) == 0 {
		r.Found = false // only empty folders: nothing to show
		r.Locations = nil
	}
	return r
}

func (s *dirSource) Clean(ctx context.Context, r *Report, _ Options) *Result {
	res := &Result{ID: s.id, Name: s.name}
	plans, _ := r.plan.([]entryPlan)
	removeEntries(ctx, plans, res)
	return res
}

// NewYarn finds Yarn 1's cache (one folder per package) and Yarn 2+'s global
// cache (one zip per package).
func NewYarn(env Env) Source {
	return &dirSource{id: "yarn", name: "Yarn cache", locate: func(context.Context) []location {
		var roots []string
		if v := env.Getenv("YARN_CACHE_FOLDER"); v != "" {
			roots = append(roots, v)
		}
		var v1 string
		switch env.GOOS {
		case "darwin":
			v1 = filepath.Join(env.cacheHome(), "Yarn")
		case "windows":
			v1 = filepath.Join(env.localAppData(), "Yarn", "Cache")
		default:
			v1 = filepath.Join(env.cacheHome(), "yarn")
		}
		// Yarn 1 keeps a versioned folder (v6) inside its cache folder.
		for _, root := range append(roots, v1) {
			matches, _ := filepath.Glob(filepath.Join(root, "v[0-9]*"))
			if len(matches) == 0 {
				roots = appendUnique(roots, root)
			}
			for _, m := range matches {
				roots = appendUnique(roots, m)
			}
		}
		roots = appendUnique(roots, filepath.Join(env.Home, ".yarn", "berry", "cache"))
		locs := make([]location, 0, len(roots))
		for _, r := range roots {
			if r == v1 {
				continue // only its versioned folders hold packages
			}
			locs = append(locs, location{root: r, skip: hidden})
		}
		return locs
	}}
}

// NewBun finds Bun's install cache.
func NewBun(env Env) Source {
	return &dirSource{id: "bun", name: "Bun cache", locate: func(context.Context) []location {
		root := env.Getenv("BUN_INSTALL_CACHE_DIR")
		if root == "" {
			base := env.Getenv("BUN_INSTALL")
			if base == "" {
				base = filepath.Join(env.Home, ".bun")
			}
			root = filepath.Join(base, "install", "cache")
		}
		return []location{{root: root}}
	}}
}
