package source

// Precise assertions found missing by mutation testing: exact counts and
// sizes, edge values, and notes that must appear only when they apply.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

func TestContentPathDigestLengths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "_cacache")
	// "AAAA" decodes to exactly 3 bytes: the shortest digest with a path.
	if p := contentPath(root, "sha512-AAAA"); p != filepath.Join(root, "content-v2", "sha512", "00", "00", "00") {
		t.Errorf("3-byte digest: %q", p)
	}
	// "AAA=" decodes to 2 bytes: too short to place under xx/yy/.
	if p := contentPath(root, "sha512-AAA="); p != "" {
		t.Errorf("2-byte digest: %q", p)
	}
	// Options after "?" are dropped; "?" right at the start leaves no digest.
	if contentPath(root, "sha512-AAAA?alg=x") != contentPath(root, "sha512-AAAA") {
		t.Error("query not stripped")
	}
	if p := contentPath(root, "sha512-?AAAA"); p != "" {
		t.Errorf("empty digest before ?: %q", p)
	}
}

func TestNPMExactCountsAndFreed(t *testing.T) {
	ctx := context.Background()
	cacheDir := t.TempDir()
	c := &fakeCache{t: t, root: filepath.Join(cacheDir, "_cacache")}
	oldIn, newIn := c.content("old content"), c.content("new content")
	oldBucket := c.bucket("aaaaaa", indexLine("old", &oldIn, oldTime))
	newBucket := c.bucket("bbbbbb", indexLine("new", &newIn, testNow))
	s := NewNPM(testEnv(t.TempDir(), "linux", map[string]string{"npm_config_cache": cacheDir}, nil))
	r := s.Scan(ctx, month)
	if r.Items != 2 || r.PrunableItems != 1 {
		t.Fatalf("items %d prunable %d, want 2 and 1", r.Items, r.PrunableItems)
	}
	planned := r.PrunableBytes
	res := s.Clean(ctx, r, month)
	if res.Freed != planned || res.Removed != 1 {
		t.Fatalf("freed %d (planned %d), removed %d", res.Freed, planned, res.Removed)
	}
	// The old bucket is gone, the new one untouched.
	if exists(oldBucket) || !exists(newBucket) {
		t.Error("wrong bucket removed")
	}
}

func TestEntryPlansOnlyWhenSomethingIsOld(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bun", "install", "cache", "pkg@1", "index.js"), "x")
	r := &Report{}
	plans := scanLocations(context.Background(), r, []location{{root: filepath.Join(home, ".bun", "install", "cache")}}, month)
	if len(plans) != 0 || r.PrunableItems != 0 || r.Items != 1 {
		t.Fatalf("plans %d, report %+v", len(plans), r)
	}
}

func TestYarnVersionedAndPlainFolders(t *testing.T) {
	home := t.TempDir()
	plain := filepath.Join(home, "yc")
	writeFile(t, filepath.Join(plain, "pkg", "x"), "x")
	locs := NewYarn(testEnv(home, "linux", map[string]string{"YARN_CACHE_FOLDER": plain}, nil)).(*dirSource).locate(context.Background())
	if locs[0].root != plain {
		t.Errorf("a cache folder without v6 is used as is: %+v", locs)
	}
	writeFile(t, filepath.Join(plain, "v6", "pkg", "x"), "x")
	locs = NewYarn(testEnv(home, "linux", map[string]string{"YARN_CACHE_FOLDER": plain}, nil)).(*dirSource).locate(context.Background())
	if locs[0].root != filepath.Join(plain, "v6") {
		t.Errorf("v6 inside is used: %+v", locs)
	}
}

func TestProjectsBiggestFirstAndSharedNote(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	for name, size := range map[string]int{"small": 100, "big": 9000, "mid": 3000} {
		p := filepath.Join(home, name)
		writeFile(t, filepath.Join(p, "node_modules", "f.js"), strings.Repeat("x", size))
		writeFile(t, filepath.Join(p, "package.json"), "{}")
		age(t, filepath.Join(p, "package.json"), oldTime)
		age(t, p, oldTime)
	}
	s := NewNodeModules(testEnv(home, "linux", nil, nil))
	r := s.Scan(ctx, month)
	ps := Projects(r, testNow)
	if len(ps) != 3 || ps[0].Bytes < ps[1].Bytes || ps[1].Bytes < ps[2].Bytes {
		t.Fatalf("not biggest first: %+v", ps)
	}
	if !strings.HasSuffix(filepath.Dir(ps[0].Path), "big") || ps[0].IdleDays < 59 {
		t.Errorf("first = %+v", ps[0])
	}
	if len(r.Notes) != 0 {
		t.Errorf("no hard links, yet notes: %v", r.Notes)
	}
}

func TestTargetsNeverGetThePnpmNote(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, "rs")
	writeFile(t, filepath.Join(p, "Cargo.toml"), "[package]")
	writeFile(t, filepath.Join(p, "target", "a"), strings.Repeat("x", 5000))
	if err := os.Link(filepath.Join(p, "target", "a"), filepath.Join(home, "elsewhere")); err != nil {
		t.Skip("hard links unavailable:", err)
	}
	age(t, filepath.Join(p, "Cargo.toml"), oldTime)
	age(t, p, oldTime)
	r := NewTargets(testEnv(home, "linux", nil, nil)).Scan(context.Background(), month)
	if r.PrunableItems != 1 || len(r.Notes) != 0 {
		t.Fatalf("report = %+v", r)
	}
}

func TestPnpmNotesAndExactFreed(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	store := filepath.Join(home, "Library", "pnpm", "store", "v10")
	writeFile(t, filepath.Join(store, "files", "aa", "unused"), strings.Repeat("x", 8000))
	pruned := false
	r := &fakeRunner{installed: map[string]bool{"pnpm": true}, handler: func(cmd string) ([]byte, error, bool) {
		switch cmd {
		case "pnpm store path":
			return []byte(store + "\n"), nil, true
		case "pnpm store prune":
			pruned = true
			_ = os.Remove(filepath.Join(store, "files", "aa", "unused"))
			return nil, nil, true
		}
		return nil, nil, false
	}}
	s := NewPnpm(testEnv(home, "darwin", nil, r))
	// With everything (--all), the "whatever their age" note doesn't apply.
	if rep := s.Scan(ctx, every); hasNote(rep, "whatever their age") || !rep.Estimate {
		t.Errorf("all: %+v", rep)
	}
	rep := s.Scan(ctx, month)
	if !hasNote(rep, "whatever their age") {
		t.Errorf("age: notes %v", rep.Notes)
	}
	before, _ := fsx.DirSize(ctx, store, fsx.Options{})
	res := s.Clean(ctx, rep, month)
	after, _ := fsx.DirSize(ctx, store, fsx.Options{})
	if !pruned || res.Freed != before.Bytes-after.Bytes || res.Freed <= 0 {
		t.Fatalf("freed %d, want %d", res.Freed, before.Bytes-after.Bytes)
	}

	// Nothing unreferenced: no estimate and no prune.
	empty := t.TempDir()
	es := filepath.Join(empty, "Library", "pnpm", "store", "v10")
	if err := os.MkdirAll(filepath.Join(es, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	r2 := &fakeRunner{installed: map[string]bool{"pnpm": true}, out: map[string]string{"pnpm store path": es}}
	if rep := NewPnpm(testEnv(empty, "darwin", nil, r2)).Scan(ctx, month); rep.Estimate || rep.PrunableBytes != 0 || len(rep.Notes) != 0 {
		t.Errorf("empty store: %+v", rep)
	}
}

func TestDockerNotesOnlyWhenTheyApply(t *testing.T) {
	ctx := context.Background()
	df := `{"Reclaimable":"0B","Size":"0B","TotalCount":"0","Type":"Images"}
{"Reclaimable":"0B","Size":"10MB","TotalCount":"1","Type":"Local Volumes"}
`
	r := &fakeRunner{installed: map[string]bool{"docker": true}, out: map[string]string{
		"docker info --format {{.OperatingSystem}}": "Docker Engine",
		"docker system df --format {{json .}}":      df,
	}}
	rep := NewDocker(testEnv(t.TempDir(), "linux", nil, r)).Scan(ctx, month)
	if rep.Estimate || rep.PrunableBytes != 0 || len(rep.Notes) != 0 {
		t.Fatalf("nothing reclaimable, yet %+v", rep)
	}
	// With --all there is no age note.
	r.out["docker system df --format {{json .}}"] = dfOutput
	rep = NewDocker(testEnv(t.TempDir(), "linux", nil, r)).Scan(ctx, every)
	if hasNote(rep, "older than") || !rep.Estimate {
		t.Errorf("all: %+v", rep)
	}
}

func TestDockerUntilFilters(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		age  time.Duration
		want string
	}{
		{time.Hour, "until=1h"},
		{30 * time.Minute, "until=30m0s"},
		{59 * time.Minute, "until=59m0s"},
	} {
		r := dockerRunner("Docker Engine")
		s := NewDocker(testEnv(t.TempDir(), "linux", nil, r))
		o := Options{Age: tc.age, Now: testNow}
		s.Clean(ctx, s.Scan(ctx, o), o)
		if !strings.Contains(strings.Join(r.calls, "|"), "container prune --force --filter "+tc.want) {
			t.Errorf("age %v: calls %v", tc.age, r.calls)
		}
	}
}

func TestDockerDesktopNoteOnlyAfterFreeing(t *testing.T) {
	ctx := context.Background()
	r := dockerRunner("Docker Desktop")
	r.handler = func(string) ([]byte, error, bool) { return []byte("Total reclaimed space: 0B\n"), nil, true }
	s := NewDocker(testEnv(t.TempDir(), "darwin", nil, r))
	if res := s.Clean(ctx, s.Scan(ctx, month), month); len(res.Notes) != 0 || res.Freed != 0 {
		t.Errorf("nothing freed, yet %+v", res)
	}
	r2 := dockerRunner("Docker Desktop")
	s2 := NewDocker(testEnv(t.TempDir(), "darwin", nil, r2))
	if res := s2.Clean(ctx, s2.Scan(ctx, month), month); len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "macOS") {
		t.Errorf("macOS note: %+v", res)
	}
}

func TestIsHexBoundaries(t *testing.T) {
	for s, want := range map[string]bool{"0": true, "9": true, "a": true, "f": true, "/": false, ":": false, "`": false, "g": false, "A": false} {
		if isHex(s) != want {
			t.Errorf("isHex(%q) = %v", s, !want)
		}
	}
	if countDeleted([]byte(strings.Repeat("f", 64)+"\n"+strings.Repeat("g", 64)+"\n")) != 1 {
		t.Error("only hex IDs count")
	}
}

func TestVersionEdges(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2", "1.2.1", -1},
		{"1.2.1", "1.2", 1},
		{"1.10", "1.9", 1},
		{"2", "10", -1},
		{"x-5", "x-5", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	// A leading "-" is part of the name, not a version separator.
	if k, v := splitVersion("-123"); k != "-123" || v != "" {
		t.Errorf("splitVersion(-123) = %q, %q", k, v)
	}
	// Equal versions keep the first one seen, deterministically.
	e1 := fsx.Entry{Path: "/c/x-1"}
	e2 := fsx.Entry{Path: "/c/x-1.0"}
	keep := keepNewest([]fsx.Entry{e1, e2})
	if len(keep) != 1 || !keep["/c/x-1"] {
		t.Errorf("keep = %v", keep)
	}
}

func TestGoPathListAndPartialEnv(t *testing.T) {
	home := t.TempDir()
	first := filepath.Join(home, "first")
	gopath := first + string(os.PathListSeparator) + filepath.Join(home, "second")
	// GOCACHE set, GOMODCACHE not, and no go command: GOPATH's first entry is used.
	locs := NewGo(testEnv(home, "linux", map[string]string{"GOCACHE": filepath.Join(home, "gc"), "GOPATH": gopath}, nil)).(*dirSource).locate(context.Background())
	if locs[0].root != filepath.Join(home, "gc") || locs[1].root != filepath.Join(first, "pkg", "mod") {
		t.Errorf("locs = %+v", locs)
	}
	// Both set: go env is not asked.
	r := &fakeRunner{installed: map[string]bool{"go": true}}
	NewGo(testEnv(home, "linux", map[string]string{"GOCACHE": filepath.Join(home, "gc"), "GOMODCACHE": filepath.Join(home, "mc")}, r)).(*dirSource).locate(context.Background())
	if len(r.calls) != 0 {
		t.Errorf("go env asked although both were set: %v", r.calls)
	}
}

func TestUVExactFreed(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	dir := filepath.Join(home, "uv")
	writeFile(t, filepath.Join(dir, "a", "b"), strings.Repeat("x", 6000))
	writeFile(t, filepath.Join(dir, "keep"), strings.Repeat("y", 3000))
	r := &fakeRunner{installed: map[string]bool{"uv": true}, handler: func(cmd string) ([]byte, error, bool) {
		if cmd == "uv cache clean" {
			_ = os.RemoveAll(filepath.Join(dir, "a"))
			return nil, nil, true
		}
		return nil, nil, false
	}}
	s := NewUV(testEnv(home, "linux", map[string]string{"UV_CACHE_DIR": dir}, r))
	before, _ := fsx.DirSize(ctx, dir, fsx.Options{})
	res := s.Clean(ctx, s.Scan(ctx, every), every)
	after, _ := fsx.DirSize(ctx, dir, fsx.Options{})
	if res.Freed != before.Bytes-after.Bytes || res.Freed <= 0 {
		t.Fatalf("freed %d, want %d", res.Freed, before.Bytes-after.Bytes)
	}
}
