package source

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

// pnpmSource is pnpm's content-addressable store and its metadata cache.
//
// Projects' node_modules hard-link files from the store, so deleting store
// files that a project uses frees nothing. pnpm's own `pnpm store prune`
// removes exactly the packages no project uses; diskdiet runs it and estimates
// beforehand what it will free from files that have no other links.
type pnpmSource struct{ env Env }

// NewPnpm finds pnpm's store and metadata cache.
func NewPnpm(env Env) Source { return &pnpmSource{env: env} }

func (s *pnpmSource) ID() string   { return "pnpm" }
func (s *pnpmSource) Name() string { return "pnpm store" }

// stores lists pnpm store folders: what `pnpm store path` reports, and the
// versioned folders (v3, v10, …) in the default location.
func (s *pnpmSource) stores(ctx context.Context) []string {
	var out []string
	if _, err := s.env.Run.LookPath("pnpm"); err == nil {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if b, err := s.env.Run.Run(ctx, "pnpm", "store", "path"); err == nil {
			if p := strings.TrimSpace(lastLine(string(b))); p != "" && filepath.IsAbs(p) && fsx.IsDir(p) {
				out = append(out, filepath.Clean(p))
			}
		}
	}
	var bases []string
	if v := s.env.Getenv("PNPM_HOME"); v != "" {
		bases = append(bases, filepath.Join(v, "store"))
	}
	switch s.env.GOOS {
	case "darwin":
		bases = append(bases, filepath.Join(s.env.Home, "Library", "pnpm", "store"))
	case "windows":
		bases = append(bases, filepath.Join(s.env.localAppData(), "pnpm", "store"))
	default:
		bases = append(bases, filepath.Join(s.env.dataHome(), "pnpm", "store"))
	}
	for _, b := range bases {
		matches, _ := filepath.Glob(filepath.Join(b, "v[0-9]*"))
		for _, m := range matches {
			if fsx.IsDir(m) {
				out = appendUnique(out, filepath.Clean(m))
			}
		}
	}
	return out
}

func (s *pnpmSource) metadataDir() string {
	if s.env.GOOS == "windows" {
		return filepath.Join(s.env.localAppData(), "pnpm-cache")
	}
	return filepath.Join(s.env.cacheHome(), "pnpm")
}

type pnpmPlan struct {
	prune    bool
	stores   []string
	metadata []entryPlan
}

func (s *pnpmSource) Scan(ctx context.Context, o Options) *Report {
	r := &Report{ID: s.ID(), Name: s.Name()}
	plan := &pnpmPlan{}
	for _, store := range s.stores(ctx) {
		r.Found = true
		r.Locations = append(r.Locations, store)
		u, _ := fsx.DirSize(ctx, store, fsx.Options{CountLinks: true})
		r.Bytes += u.Bytes
		r.Items += u.Files
		plan.stores = append(plan.stores, store)
		files, _ := fsx.DirSize(ctx, filepath.Join(store, "files"), fsx.Options{CountLinks: true})
		// Only files no project links to can be pruned; the store's folders
		// alone (4 kB each on Linux) are not something to free.
		if files.ExclusiveFiles > 0 {
			r.PrunableBytes += files.Exclusive
			r.Estimate = true
			plan.prune = true
		}
	}
	if plan.prune {
		if _, err := s.env.Run.LookPath("pnpm"); err != nil {
			plan.prune = false
			r.PrunableBytes = 0
			r.Estimate = false
			r.note("pnpm isn't installed, so its store can't be pruned safely")
		} else if o.Age > 0 {
			r.note("pnpm removes packages no project uses, whatever their age")
		}
	}
	plan.metadata = scanLocations(ctx, r, []location{{root: s.metadataDir(), files: true}}, o)
	r.plan = plan
	return r
}

func (s *pnpmSource) Clean(ctx context.Context, r *Report, _ Options) *Result {
	res := &Result{ID: s.ID(), Name: s.Name()}
	plan, _ := r.plan.(*pnpmPlan)
	if plan == nil {
		return res
	}
	if plan.prune {
		before := s.measure(ctx, plan.stores)
		ctx2, cancel := context.WithTimeout(ctx, 30*time.Minute)
		_, err := s.env.Run.Run(ctx2, "pnpm", "store", "prune")
		cancel()
		res.fail(err)
		if freed := before - s.measure(ctx, plan.stores); freed > 0 {
			res.Freed += freed
		}
		if err == nil {
			res.Notes = append(res.Notes, "ran pnpm store prune")
		}
	}
	removeEntries(ctx, plan.metadata, res)
	return res
}

func (s *pnpmSource) measure(ctx context.Context, stores []string) int64 {
	var n int64
	for _, st := range stores {
		u, _ := fsx.DirSize(ctx, st, fsx.Options{})
		n += u.Bytes
	}
	return n
}

func lastLine(s string) string {
	s = strings.TrimRight(s, "\r\n")
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
