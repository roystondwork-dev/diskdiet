package source

import (
	"context"
	"path/filepath"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

// Demo mode (DISKDIET_DEMO=1) shows made-up but realistic caches and removes
// nothing, for screenshots, recordings and trying the app safely.

// DemoEnabled reports whether demo mode is on.
func DemoEnabled(getenv func(string) string) bool {
	v := getenv("DISKDIET_DEMO")
	return v != "" && v != "0" && v != "false"
}

type demoSource struct {
	id, name string
	report   Report
	projects []demoProject
}

type demoProject struct {
	path  string
	bytes int64
	idle  time.Duration
}

func (s *demoSource) ID() string   { return s.id }
func (s *demoSource) Name() string { return s.name }

// Scan scales what is old by the age asked for: everything with --all, more
// with a shorter age.
func (s *demoSource) Scan(_ context.Context, o Options) *Report {
	r := s.report
	r.ID, r.Name, r.Found = s.id, s.name, true
	r.Locations = append([]string{}, s.report.Locations...)
	r.Notes = append([]string{}, s.report.Notes...)
	if len(s.projects) > 0 {
		var plan []entryPlan
		r.PrunableBytes, r.PrunableItems = 0, 0
		for _, p := range s.projects {
			mod := o.Now.Add(-p.idle)
			if !o.Old(mod) {
				continue
			}
			e := fsx.Entry{Path: p.path, Usage: fsx.Usage{Bytes: p.bytes, Exclusive: p.bytes, Files: 1}, Modified: mod}
			plan = append(plan, entryPlan{root: filepath.Dir(p.path), entries: []fsx.Entry{e}})
			r.PrunableBytes += p.bytes
			r.PrunableItems++
		}
		r.plan = plan
		return &r
	}
	switch {
	case o.Age <= 0:
		r.PrunableBytes, r.PrunableItems = r.Bytes, r.Items
	case o.Age < 14*24*time.Hour:
		r.PrunableBytes = r.PrunableBytes * 13 / 10
		r.PrunableItems = r.PrunableItems * 13 / 10
	case o.Age > 60*24*time.Hour:
		r.PrunableBytes /= 2
		r.PrunableItems /= 2
	}
	r.PrunableBytes = min(r.PrunableBytes, r.Bytes)
	return &r
}

// Clean pretends: it reports what the scan planned as freed.
func (s *demoSource) Clean(_ context.Context, r *Report, _ Options) *Result {
	time.Sleep(400 * time.Millisecond) // long enough to see progress
	return &Result{ID: s.id, Name: s.name, Freed: r.PrunableBytes, Removed: r.PrunableItems}
}

const gB = int64(1_000_000_000)

// Demo returns made-up sources under home.
func Demo(home string) []Source {
	day := 24 * time.Hour
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	return []Source{
		&demoSource{id: "npm", name: "npm cache", report: Report{Bytes: 41*gB + 3e8, Items: 48210,
			PrunableBytes: 33*gB + 8e8, PrunableItems: 39102, Locations: []string{j(".npm")}}},
		&demoSource{id: "yarn", name: "Yarn cache", report: Report{Bytes: 2*gB + 4e8, Items: 3120,
			PrunableBytes: 1*gB + 9e8, PrunableItems: 2480, Locations: []string{j("Library", "Caches", "Yarn", "v6")}}},
		&demoSource{id: "pnpm", name: "pnpm store", report: Report{Bytes: 6*gB + 1e8, Items: 220004,
			PrunableBytes: 1*gB + 4e8, PrunableItems: 1, Estimate: true, Locations: []string{j("Library", "pnpm", "store", "v10")},
			Notes: []string{"pnpm removes packages no project uses, whatever their age"}}},
		&demoSource{id: "go", name: "Go caches", report: Report{Bytes: 9*gB + 2e8, Items: 61204,
			PrunableBytes: 4*gB + 7e8, PrunableItems: 30188, Locations: []string{j("Library", "Caches", "go-build"), j("go", "pkg", "mod")}}},
		&demoSource{id: "cargo", name: "Cargo cache", report: Report{Bytes: 3*gB + 6e8, Items: 9120,
			PrunableBytes: 2*gB + 2e8, PrunableItems: 5433, Locations: []string{j(".cargo")}}},
		&demoSource{id: "pip", name: "pip cache", report: Report{Bytes: 1*gB + 8e8, Items: 2210,
			PrunableBytes: 1*gB + 3e8, PrunableItems: 1730, Locations: []string{j("Library", "Caches", "pip")}}},
		&demoSource{id: "playwright", name: "Playwright browsers", report: Report{Bytes: 2*gB + 9e8, Items: 6,
			PrunableBytes: 1*gB + 7e8, PrunableItems: 3, Locations: []string{j("Library", "Caches", "ms-playwright")}}},
		&demoSource{id: "docker", name: "Docker", report: Report{Bytes: 14*gB + 6e8, Items: 63,
			PrunableBytes: 9*gB + 2e8, PrunableItems: 1, Estimate: true, Locations: []string{"docker (Docker Desktop)"},
			Notes: []string{"3.1 GB in unused volumes is kept; add --volumes to remove them (volumes can hold databases)"}}},
		&demoSource{id: "node_modules", name: "node_modules", report: Report{Bytes: 11*gB + 2e8, Items: 41,
			Locations: []string{home}}, projects: []demoProject{
			{j("code", "shop-app", "node_modules"), 1*gB + 9e8, 140 * day},
			{j("code", "old-landing", "node_modules"), 1*gB + 2e8, 320 * day},
			{j("code", "hackathon-2025", "node_modules"), 9e8, 260 * day},
			{j("code", "design-system", "node_modules"), 8e8, 45 * day},
			{j("code", "api", "node_modules"), 7e8, 12 * day},
			{j("code", "blog", "node_modules"), 6e8, 95 * day},
			{j("code", "dashboard", "node_modules"), 1*gB + 5e8, 2 * day},
		}},
		&demoSource{id: "target", name: "target folders", report: Report{Bytes: 4*gB + 4e8, Items: 3,
			Locations: []string{home}}, projects: []demoProject{
			{j("code", "rust-cli", "target"), 2*gB + 6e8, 70 * day},
			{j("code", "spring-api", "target"), 4e8, 5 * day},
		}},
	}
}
