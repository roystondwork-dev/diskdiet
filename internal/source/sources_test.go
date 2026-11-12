package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

func TestAllAndIDs(t *testing.T) {
	srcs := All(testEnv(t.TempDir(), "linux", nil, nil))
	ids := IDs()
	if len(srcs) != len(ids) {
		t.Fatalf("%d sources, %d ids", len(srcs), len(ids))
	}
	for i, s := range srcs {
		if s.ID() != ids[i] || s.Name() == "" {
			t.Errorf("source %d: id %q, want %q", i, s.ID(), ids[i])
		}
	}
	env, err := SystemEnv()
	if err != nil || env.Home == "" || env.GOOS != runtime.GOOS || env.Run == nil {
		t.Errorf("SystemEnv = %+v, %v", env, err)
	}
}

func TestOptionsOld(t *testing.T) {
	if !every.Old(testNow) {
		t.Error("zero age: everything is old")
	}
	if !month.Old(oldTime) || month.Old(testNow) {
		t.Error("30d age misjudged")
	}
}

func TestEnvFolders(t *testing.T) {
	home := filepath.Join(string(filepath.Separator)+"home", "u")
	vars := map[string]string{"XDG_CACHE_HOME": "/xc", "XDG_DATA_HOME": "/xd", "LOCALAPPDATA": `C:\lad`}
	cases := []struct {
		goos      string
		vars      map[string]string
		cache, dt string
	}{
		{"darwin", nil, filepath.Join(home, "Library", "Caches"), filepath.Join(home, "Library")},
		{"windows", vars, `C:\lad`, `C:\lad`},
		{"linux", vars, "/xc", "/xd"},
		{"linux", nil, filepath.Join(home, ".cache"), filepath.Join(home, ".local", "share")},
	}
	for _, c := range cases {
		e := testEnv(home, c.goos, c.vars, nil)
		if e.cacheHome() != c.cache || e.dataHome() != c.dt {
			t.Errorf("%s: cache %q data %q", c.goos, e.cacheHome(), e.dataHome())
		}
	}
}

func TestYarn(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	v6 := filepath.Join(home, ".cache", "yarn", "v6")
	writeFile(t, filepath.Join(v6, "npm-old-1.0.0", "index.js"), "old")
	writeFile(t, filepath.Join(v6, "npm-new-1.0.0", "index.js"), "new")
	writeFile(t, filepath.Join(v6, ".tmp", "x"), "tmp")
	age(t, filepath.Join(v6, "npm-old-1.0.0"), oldTime)
	berry := filepath.Join(home, ".yarn", "berry", "cache")
	writeFile(t, filepath.Join(berry, "old.zip"), "zip")
	age(t, filepath.Join(berry, "old.zip"), oldTime)

	s := NewYarn(testEnv(home, "linux", nil, nil))
	r := s.Scan(ctx, month)
	if !r.Found || r.Items != 3 || r.PrunableItems != 2 || len(r.Locations) != 2 {
		t.Fatalf("report = %+v", r)
	}
	res := s.Clean(ctx, r, month)
	if res.Removed != 2 || len(res.Errors) > 0 {
		t.Fatalf("result = %+v", res)
	}
	if exists(filepath.Join(v6, "npm-old-1.0.0")) || !exists(filepath.Join(v6, "npm-new-1.0.0")) || !exists(filepath.Join(v6, ".tmp")) {
		t.Error("wrong entries removed")
	}
}

func TestYarnLocations(t *testing.T) {
	home := t.TempDir()
	custom := t.TempDir()
	writeFile(t, filepath.Join(custom, "pkg", "a"), "x")
	// The default Yarn folder exists but has no versioned folder: skipped.
	writeFile(t, filepath.Join(home, "Library", "Caches", "Yarn", "stray"), "x")
	s := NewYarn(testEnv(home, "darwin", map[string]string{"YARN_CACHE_FOLDER": custom}, nil))
	r := s.Scan(context.Background(), every)
	if len(r.Locations) != 1 || r.Locations[0] != custom || r.Items != 1 {
		t.Errorf("report = %+v", r)
	}

	win := filepath.Join(home, "lad", "Yarn", "Cache", "v6")
	writeFile(t, filepath.Join(win, "pkg", "a"), "x")
	s = NewYarn(testEnv(home, "windows", map[string]string{"LOCALAPPDATA": filepath.Join(home, "lad")}, nil))
	if r := s.Scan(context.Background(), every); len(r.Locations) != 1 || r.Locations[0] != win {
		t.Errorf("windows report = %+v", r)
	}
}

func TestBun(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	def := filepath.Join(home, ".bun", "install", "cache")
	writeFile(t, filepath.Join(def, "react@18.2.0@@@1", "index.js"), "x")
	r := NewBun(testEnv(home, "linux", nil, nil)).Scan(ctx, every)
	if !r.Found || r.Locations[0] != def || r.PrunableItems != 1 {
		t.Errorf("default: %+v", r)
	}
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "install", "cache", "a", "b"), "x")
	r = NewBun(testEnv(home, "linux", map[string]string{"BUN_INSTALL": base}, nil)).Scan(ctx, every)
	if r.Locations[0] != filepath.Join(base, "install", "cache") {
		t.Errorf("BUN_INSTALL: %+v", r)
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), "x")
	r = NewBun(testEnv(home, "linux", map[string]string{"BUN_INSTALL_CACHE_DIR": dir}, nil)).Scan(ctx, month)
	if r.Locations[0] != dir || r.PrunableItems != 0 || r.Items != 1 {
		t.Errorf("BUN_INSTALL_CACHE_DIR: %+v", r)
	}
	// Nothing installed.
	if r := NewBun(testEnv(t.TempDir(), "linux", nil, nil)).Scan(ctx, every); r.Found {
		t.Error("missing bun cache reported as found")
	}
}

func TestRemoveEntriesCanceled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a"), "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := &Result{}
	removeEntries(ctx, []entryPlan{{root: root, entries: []fsx.Entry{{Path: filepath.Join(root, "a")}}}}, res)
	if len(res.Errors) != 1 || !exists(filepath.Join(root, "a")) {
		t.Errorf("canceled removal: %+v", res)
	}
	res = &Result{}
	removeEntries(context.Background(), []entryPlan{{root: root, entries: []fsx.Entry{{Path: t.TempDir()}}}}, res)
	if len(res.Errors) != 1 {
		t.Errorf("outside-root removal should fail: %+v", res)
	}
}

func TestPnpm(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	store := filepath.Join(t.TempDir(), "store", "v10")
	writeFile(t, filepath.Join(store, "files", "00", "unused"), strings.Repeat("x", 5000))
	writeFile(t, filepath.Join(store, "index.db"), "db")
	meta := filepath.Join(home, ".cache", "pnpm", "metadata", "registry", "lodash.json")
	writeFile(t, meta, "{}")
	age(t, meta, oldTime)

	r := &fakeRunner{installed: map[string]bool{"pnpm": true}, out: map[string]string{
		"pnpm store path": "some warning\n" + store + "\n",
	}}
	r.handler = func(cmd string) ([]byte, error, bool) {
		if cmd == "pnpm store prune" {
			_ = os.Remove(filepath.Join(store, "files", "00", "unused"))
			return []byte("Removed 1 package"), nil, true
		}
		return nil, nil, false
	}
	s := NewPnpm(testEnv(home, "linux", nil, r))
	rep := s.Scan(ctx, month)
	if !rep.Found || !rep.Estimate || rep.PrunableBytes == 0 || rep.PrunableItems != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if !hasNote(rep, "whatever their age") {
		t.Errorf("notes = %v", rep.Notes)
	}
	res := s.Clean(ctx, rep, month)
	if len(res.Errors) > 0 || res.Freed == 0 || res.Removed != 1 || len(res.Notes) != 1 {
		t.Fatalf("result = %+v", res)
	}
	if exists(meta) {
		t.Error("old metadata kept")
	}

	// A failing prune is reported.
	writeFile(t, filepath.Join(store, "files", "00", "unused2"), "x")
	r.handler = nil
	r.errs = map[string]error{"pnpm store prune": errors.New("boom")}
	rep = s.Scan(ctx, month)
	if res := s.Clean(ctx, rep, month); len(res.Errors) != 1 || len(res.Notes) != 0 {
		t.Errorf("failed prune result = %+v", res)
	}
	if res := s.Clean(ctx, &Report{}, month); res.Freed != 0 {
		t.Error("no plan, yet freed")
	}
}

func TestPnpmNotInstalled(t *testing.T) {
	home := t.TempDir()
	store := filepath.Join(home, ".local", "share", "pnpm", "store", "v10")
	writeFile(t, filepath.Join(store, "files", "aa", "x"), "x")
	rep := NewPnpm(testEnv(home, "linux", nil, nil)).Scan(context.Background(), month)
	if !rep.Found || rep.PrunableBytes != 0 || rep.Estimate || !hasNote(rep, "isn't installed") {
		t.Errorf("report = %+v", rep)
	}
}

func TestPnpmDefaultStores(t *testing.T) {
	home := t.TempDir()
	cases := []struct {
		goos string
		vars map[string]string
		path string
	}{
		{"darwin", nil, filepath.Join(home, "Library", "pnpm", "store", "v3")},
		{"windows", map[string]string{"LOCALAPPDATA": filepath.Join(home, "lad")}, filepath.Join(home, "lad", "pnpm", "store", "v10")},
		{"linux", map[string]string{"PNPM_HOME": filepath.Join(home, "ph")}, filepath.Join(home, "ph", "store", "v10")},
	}
	for _, c := range cases {
		if err := os.MkdirAll(c.path, 0o755); err != nil {
			t.Fatal(err)
		}
		s := &pnpmSource{env: testEnv(home, c.goos, c.vars, nil)}
		got := s.stores(context.Background())
		found := false
		for _, g := range got {
			found = found || g == c.path
		}
		if !found {
			t.Errorf("%s: stores = %v, want %s", c.goos, got, c.path)
		}
	}
	s := &pnpmSource{env: testEnv(home, "windows", map[string]string{"LOCALAPPDATA": "L"}, nil)}
	if s.metadataDir() != filepath.Join("L", "pnpm-cache") {
		t.Errorf("windows metadata dir = %s", s.metadataDir())
	}
}

func TestLastLine(t *testing.T) {
	if lastLine("a\nb\r\n") != "b" || lastLine("only") != "only" {
		t.Error("lastLine")
	}
}

const dfOutput = `{"Active":"2","Reclaimable":"1GB (50%)","Size":"2GB","TotalCount":"5","Type":"Images"}
{"Active":"1","Reclaimable":"50MB (50%)","Size":"100MB","TotalCount":"2","Type":"Containers"}
{"Active":"1","Reclaimable":"300MB (60%)","Size":"500MB","TotalCount":"3","Type":"Local Volumes"}
{"Active":"0","Reclaimable":"1GB","Size":"1GB","TotalCount":"10","Type":"Build Cache"}
not json
`

func dockerRunner(osName string) *fakeRunner {
	return &fakeRunner{
		installed: map[string]bool{"docker": true},
		out: map[string]string{
			"docker info --format {{.OperatingSystem}}": osName + "\n",
			"docker system df --format {{json .}}":      dfOutput,
		},
		handler: func(cmd string) ([]byte, error, bool) {
			switch {
			case strings.HasPrefix(cmd, "docker container prune"):
				return []byte("Deleted Containers:\n" + strings.Repeat("a", 64) + "\n\nTotal reclaimed space: 50MB\n"), nil, true
			case strings.HasPrefix(cmd, "docker image prune"):
				return []byte("Deleted Images:\nuntagged: x\ndeleted: sha256:abc\n\nTotal reclaimed space: 1.5 GB\n"), nil, true
			case strings.HasPrefix(cmd, "docker builder prune"):
				return nil, errors.New("builder failed"), true
			case strings.HasPrefix(cmd, "docker volume prune"):
				return []byte("Total reclaimed space: 300MB\n"), nil, true
			}
			return nil, nil, false
		},
	}
}

func TestDockerScanAndClean(t *testing.T) {
	ctx := context.Background()
	r := dockerRunner("Docker Desktop")
	s := NewDocker(testEnv(t.TempDir(), "windows", nil, r))
	rep := s.Scan(ctx, month)
	if !rep.Found || rep.Unavailable || rep.Items != 20 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Bytes != 3_600_000_000 || rep.PrunableBytes != 2_050_000_000 || !rep.Estimate {
		t.Errorf("bytes %d prunable %d", rep.Bytes, rep.PrunableBytes)
	}
	if !hasNote(rep, "--volumes") || !hasNote(rep, "older than 30d") {
		t.Errorf("notes = %v", rep.Notes)
	}
	res := s.Clean(ctx, rep, month)
	if res.Freed != 1_550_000_000 || res.Removed != 2 || len(res.Errors) != 1 {
		t.Errorf("result = %+v", res)
	}
	if len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "WSL") {
		t.Errorf("notes = %v", res.Notes)
	}
	joined := strings.Join(r.calls, "|")
	if !strings.Contains(joined, "container prune --force --filter until=720h") || strings.Contains(joined, "volume prune") {
		t.Errorf("calls = %v", r.calls)
	}
}

func TestDockerVolumesAndShortAge(t *testing.T) {
	ctx := context.Background()
	r := dockerRunner("Docker Desktop")
	s := NewDocker(testEnv(t.TempDir(), "darwin", nil, r))
	o := Options{Age: 30 * time.Minute, Now: testNow, Volumes: true}
	rep := s.Scan(ctx, o)
	if rep.PrunableBytes != 2_350_000_000 || hasNote(rep, "--volumes") {
		t.Errorf("report = %+v", rep)
	}
	res := s.Clean(ctx, rep, o)
	joined := strings.Join(r.calls, "|")
	if !strings.Contains(joined, "until=30m0s") || !strings.Contains(joined, "volume prune --all --force") {
		t.Errorf("calls = %v", r.calls)
	}
	if len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "macOS") {
		t.Errorf("notes = %v", res.Notes)
	}

	// --all: no filter.
	r = dockerRunner("Ubuntu 24.04")
	s = NewDocker(testEnv(t.TempDir(), "linux", nil, r))
	rep = s.Scan(ctx, every)
	res = s.Clean(ctx, rep, every)
	if strings.Contains(strings.Join(r.calls, "|"), "until=") || len(res.Notes) != 0 {
		t.Errorf("calls = %v notes = %v", r.calls, res.Notes)
	}
	if res := s.Clean(ctx, &Report{}, every); len(r.calls) == 0 || res.Freed != 0 {
		t.Error("clean without plan")
	}
}

func TestDockerUnavailable(t *testing.T) {
	ctx := context.Background()
	if rep := NewDocker(testEnv(t.TempDir(), "linux", nil, nil)).Scan(ctx, month); rep.Found {
		t.Error("docker not installed but found")
	}
	r := &fakeRunner{installed: map[string]bool{"docker": true}}
	rep := NewDocker(testEnv(t.TempDir(), "linux", nil, r)).Scan(ctx, month)
	if !rep.Found || !rep.Unavailable || !hasNote(rep, "not running") {
		t.Errorf("not running: %+v", rep)
	}
	r = &fakeRunner{installed: map[string]bool{"docker": true}, out: map[string]string{"docker info --format {{.OperatingSystem}}": "x"},
		errs: map[string]error{"docker system df --format {{json .}}": errors.New("df failed")}}
	rep = NewDocker(testEnv(t.TempDir(), "linux", nil, r)).Scan(ctx, month)
	if rep.Error == "" {
		t.Errorf("df failure not reported: %+v", rep)
	}
}

func TestCountDeleted(t *testing.T) {
	out := "deleted: sha256:1\n" + strings.Repeat("f", 64) + "\n" + strings.Repeat("g", 64) + "\nTotal reclaimed space: 0B\n"
	if n := countDeleted([]byte(out)); n != 2 {
		t.Errorf("countDeleted = %d, want 2", n)
	}
}

func TestNodeModules(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	oldProj := filepath.Join(home, "code", "old")
	newProj := filepath.Join(home, "code", "new")
	writeFile(t, filepath.Join(oldProj, "node_modules", "dep", "index.js"), strings.Repeat("x", 4000))
	writeFile(t, filepath.Join(oldProj, "node_modules", "dep", "node_modules", "inner", "i.js"), "x")
	writeFile(t, filepath.Join(oldProj, "package.json"), "{}")
	age(t, filepath.Join(oldProj, "package.json"), oldTime)
	age(t, oldProj, oldTime)
	writeFile(t, filepath.Join(newProj, "node_modules", "dep", "index.js"), "x")
	writeFile(t, filepath.Join(newProj, "package.json"), "{}")
	writeFile(t, filepath.Join(home, ".hidden", "p", "node_modules", "x"), "x")
	writeFile(t, filepath.Join(home, "Library", "p", "node_modules", "x"), "x")

	s := NewNodeModules(testEnv(home, "darwin", nil, nil))
	if s.ID() != "node_modules" || s.Name() != "node_modules" {
		t.Fatal("id/name")
	}
	rep := s.Scan(ctx, month)
	if rep.Items != 2 || rep.PrunableItems != 1 || rep.PrunableBytes == 0 || rep.Locations[0] != home {
		t.Fatalf("report = %+v", rep)
	}
	ps := Projects(rep, testNow)
	if len(ps) != 1 || ps[0].Path != filepath.Join(oldProj, "node_modules") || ps[0].Idle < 59*24*time.Hour {
		t.Fatalf("projects = %+v", ps)
	}
	res := s.Clean(ctx, rep, month)
	if res.Removed != 1 || len(res.Errors) > 0 {
		t.Fatalf("result = %+v", res)
	}
	if exists(filepath.Join(oldProj, "node_modules")) || !exists(filepath.Join(newProj, "node_modules")) {
		t.Error("wrong node_modules removed")
	}
}

func TestNodeModulesProjectsOption(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "node_modules", "x"), "x")
	o := Options{Now: testNow, Projects: []string{root, filepath.Join(root, "missing")}}
	rep := NewNodeModules(testEnv(t.TempDir(), "linux", nil, nil)).Scan(context.Background(), o)
	if rep.Items != 1 || len(rep.Locations) != 1 || !hasNote(rep, "not a folder") {
		t.Errorf("report = %+v", rep)
	}
}

func TestNodeModulesSharedNote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("link counts are not used for node_modules on Windows")
	}
	root := t.TempDir()
	store := t.TempDir()
	writeFile(t, filepath.Join(store, "f"), strings.Repeat("x", 8000))
	if err := os.MkdirAll(filepath.Join(root, "p", "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(store, "f"), filepath.Join(root, "p", "node_modules", "f")); err != nil {
		t.Skip(err)
	}
	rep := NewNodeModules(testEnv(t.TempDir(), "linux", nil, nil)).Scan(context.Background(), Options{Now: testNow, Projects: []string{root}})
	if !hasNote(rep, "hard-linked") {
		t.Errorf("notes = %v", rep.Notes)
	}
}

func TestLastTouched(t *testing.T) {
	if !lastTouched(filepath.Join(t.TempDir(), "missing"), "node_modules").IsZero() {
		t.Error("missing project should have zero time")
	}
	p := t.TempDir()
	writeFile(t, filepath.Join(p, "src", "a.js"), "x")
	writeFile(t, filepath.Join(p, "node_modules", "x"), "x")
	age(t, filepath.Join(p, "src"), oldTime)
	age(t, p, oldTime)
	// node_modules is new but ignored.
	if got := lastTouched(p, "node_modules"); !got.Equal(oldTime) {
		t.Errorf("lastTouched = %v, want %v", got, oldTime)
	}
}

func TestKeepProjects(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	projs := make([]string, 0, 2)
	for _, name := range []string{"a", "b"} {
		p := filepath.Join(home, name)
		writeFile(t, filepath.Join(p, "node_modules", "x.js"), strings.Repeat("x", 5000))
		writeFile(t, filepath.Join(p, "package.json"), "{}")
		age(t, filepath.Join(p, "package.json"), oldTime)
		age(t, p, oldTime)
		projs = append(projs, p)
	}
	s := NewNodeModules(testEnv(home, "linux", nil, nil))
	rep := s.Scan(ctx, month)
	if rep.PrunableItems != 2 {
		t.Fatalf("report = %+v", rep)
	}
	before := rep.PrunableBytes
	KeepProjects(rep, nil) // no-op
	KeepProjects(rep, []string{filepath.Join(projs[0], "node_modules")})
	if rep.PrunableItems != 1 || rep.PrunableBytes >= before {
		t.Fatalf("after keep = %+v", rep)
	}
	res := s.Clean(ctx, rep, month)
	if res.Removed != 1 || !exists(filepath.Join(projs[0], "node_modules")) || exists(filepath.Join(projs[1], "node_modules")) {
		t.Fatalf("result = %+v", res)
	}
	KeepProjects(&Report{}, []string{"x"}) // reports without a node_modules plan are ignored
}

func TestProjectSourcesShareOneWalk(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "web", "node_modules", "x.js"), "x")
	writeFile(t, filepath.Join(home, "rs", "Cargo.toml"), "[package]")
	writeFile(t, filepath.Join(home, "rs", "target", "app"), "x")
	env := testEnv(home, "linux", nil, nil)
	f := newProjectFinder(env)
	nm, tg := newNodeModules(env, f), newTargets(env, f)
	var wg sync.WaitGroup
	var rn, rt *Report
	wg.Go(func() { rn = nm.Scan(context.Background(), month) })
	wg.Go(func() { rt = tg.Scan(context.Background(), month) })
	wg.Wait()
	if rn.Items != 1 || rt.Items != 1 || f.walks != 1 {
		t.Fatalf("node_modules %d, target %d, walks %d", rn.Items, rt.Items, f.walks)
	}
	// A new scan walks again.
	later := month
	later.Now = later.Now.Add(time.Minute)
	nm.Scan(context.Background(), later)
	if f.walks != 2 {
		t.Fatalf("walks = %d", f.walks)
	}
	// All shares one finder between the two.
	var got []*projectFinder
	for _, s := range All(env) {
		if ps, ok := s.(*projectSource); ok {
			got = append(got, ps.finder)
		}
	}
	if len(got) != 2 || got[0] != got[1] {
		t.Fatal("All should share the finder")
	}
}
