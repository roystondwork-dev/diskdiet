package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkOld writes a file and makes it (and its folder) old.
func mkOld(t *testing.T, path string) {
	t.Helper()
	writeFile(t, path, strings.Repeat("x", 3000))
	age(t, path, oldTime)
	age(t, filepath.Dir(path), oldTime)
}

func mkNew(t *testing.T, path string) {
	t.Helper()
	writeFile(t, path, strings.Repeat("y", 3000))
}

// scanClean scans with a 30-day age, checks the plan, cleans, and returns the
// report.
func scanClean(t *testing.T, s Source, wantItems, wantPrunable int) *Report {
	t.Helper()
	ctx := context.Background()
	r := s.Scan(ctx, month)
	if !r.Found || r.Items != wantItems || r.PrunableItems != wantPrunable {
		t.Fatalf("%s: found=%v items=%d prunable=%d notes=%v, want %d/%d", s.ID(), r.Found, r.Items, r.PrunableItems, r.Notes, wantItems, wantPrunable)
	}
	res := s.Clean(ctx, r, month)
	if len(res.Errors) > 0 || res.Removed != wantPrunable {
		t.Fatalf("%s: clean = %+v", s.ID(), res)
	}
	return r
}

func TestPip(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".cache", "pip")
	mkOld(t, filepath.Join(root, "http-v2", "a", "b", "old.body"))
	mkNew(t, filepath.Join(root, "wheels", "x", "new.whl"))
	scanClean(t, NewPip(testEnv(home, "linux", nil, nil)), 2, 1)
	if exists(filepath.Join(root, "http-v2", "a", "b", "old.body")) || !exists(filepath.Join(root, "wheels", "x", "new.whl")) {
		t.Error("wrong pip files removed")
	}
	// Defaults per OS and the env override.
	for goos, want := range map[string]string{
		"darwin":  filepath.Join(home, "Library", "Caches", "pip"),
		"windows": filepath.Join(home, "AppData", "Local", "pip", "Cache"),
	} {
		locs := NewPip(testEnv(home, goos, nil, nil)).(*dirSource).locate(context.Background())
		if locs[0].root != want {
			t.Errorf("%s: %s, want %s", goos, locs[0].root, want)
		}
	}
	locs := NewPip(testEnv(home, "linux", map[string]string{"PIP_CACHE_DIR": "/x/pip"}, nil)).(*dirSource).locate(context.Background())
	if locs[0].root != "/x/pip" {
		t.Error(locs[0].root)
	}
}

func TestEmptyCacheIsNotFound(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".m2", "repository"), 0o755); err != nil {
		t.Fatal(err)
	}
	if r := NewMaven(testEnv(home, "linux", nil, nil)).Scan(context.Background(), month); r.Found || r.Locations != nil {
		t.Errorf("empty repository reported: %+v", r)
	}
}

func TestCargo(t *testing.T) {
	home := t.TempDir()
	c := filepath.Join(home, ".cargo")
	mkOld(t, filepath.Join(c, "registry", "src", "index.crates.io-1", "serde-1.0.0", "lib.rs"))
	mkNew(t, filepath.Join(c, "registry", "src", "index.crates.io-1", "serde-1.0.200", "lib.rs"))
	mkOld(t, filepath.Join(c, "registry", "cache", "index.crates.io-1", "serde-1.0.0.crate"))
	mkOld(t, filepath.Join(c, "git", "checkouts", "repo-abc", "1234567", "src.rs"))
	mkOld(t, filepath.Join(c, "git", "db", "repo-abc", "HEAD"))
	r := scanClean(t, NewCargo(testEnv(home, "linux", nil, nil)), 5, 4)
	if len(r.Locations) != 1 || r.Locations[0] != c {
		t.Errorf("locations = %v", r.Locations)
	}
	if !exists(filepath.Join(c, "registry", "src", "index.crates.io-1", "serde-1.0.200")) ||
		exists(filepath.Join(c, "registry", "src", "index.crates.io-1", "serde-1.0.0")) {
		t.Error("wrong crates removed")
	}
	locs := NewCargo(testEnv(home, "linux", map[string]string{"CARGO_HOME": "/x/cargo"}, nil)).(*dirSource).locate(context.Background())
	if locs[0].show != "/x/cargo" {
		t.Errorf("CARGO_HOME ignored: %+v", locs[0])
	}
}

func TestGo(t *testing.T) {
	home := t.TempDir()
	gocache := filepath.Join(home, "gocache")
	mod := filepath.Join(home, "go", "pkg", "mod")
	mkOld(t, filepath.Join(gocache, "ab", "abcd-a"))
	mkNew(t, filepath.Join(gocache, "cd", "cdef-a"))
	mkOld(t, filepath.Join(mod, "github.com", "x", "y@v1.0.0", "y.go"))
	mkNew(t, filepath.Join(mod, "github.com", "x", "y@v1.2.0", "y.go"))
	mkOld(t, filepath.Join(mod, "cache", "download", "github.com", "x", "y", "@v", "v1.0.0.zip"))
	// Module folders are read-only, as the go command leaves them.
	if err := os.Chmod(filepath.Join(mod, "github.com", "x", "y@v1.0.0"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(mod, "github.com", "x", "y@v1.0.0"), 0o755) })
	r := &fakeRunner{installed: map[string]bool{"go": true},
		out: map[string]string{"go env GOCACHE GOMODCACHE": gocache + "\n" + mod + "\n"}}
	scanClean(t, NewGo(testEnv(home, "linux", nil, r)), 5, 3)
	if exists(filepath.Join(mod, "github.com", "x", "y@v1.0.0")) || !exists(filepath.Join(mod, "github.com", "x", "y@v1.2.0")) {
		t.Error("wrong module versions removed")
	}

	// Without the go command: defaults, GOPATH and GOCACHE=off.
	env := testEnv(home, "darwin", map[string]string{"GOPATH": "/gp" + string(os.PathListSeparator) + "/other", "GOCACHE": "off"}, nil)
	locs := NewGo(env).(*dirSource).locate(context.Background())
	if len(locs) != 2 || locs[0].root != filepath.Join("/gp", "pkg", "mod") {
		t.Errorf("locs = %+v", locs)
	}
	for goos, want := range map[string]string{
		"darwin":  filepath.Join(home, "Library", "Caches", "go-build"),
		"windows": filepath.Join(home, "AppData", "Local", "go-build"),
		"linux":   filepath.Join(home, ".cache", "go-build"),
	} {
		locs := NewGo(testEnv(home, goos, nil, nil)).(*dirSource).locate(context.Background())
		if locs[0].root != want || locs[1].root != filepath.Join(home, "go", "pkg", "mod") {
			t.Errorf("%s: %+v", goos, locs)
		}
	}
	// go env failing falls back to defaults.
	bad := &fakeRunner{installed: map[string]bool{"go": true}, errs: map[string]error{"go env GOCACHE GOMODCACHE": errors.New("x")}}
	if locs := NewGo(testEnv(home, "linux", nil, bad)).(*dirSource).locate(context.Background()); locs[0].root != filepath.Join(home, ".cache", "go-build") {
		t.Errorf("fallback: %+v", locs)
	}
}

func TestGradle(t *testing.T) {
	home := t.TempDir()
	g := filepath.Join(home, ".gradle")
	mkOld(t, filepath.Join(g, "wrapper", "dists", "gradle-7.6-bin", "x", "gradle.zip"))
	mkNew(t, filepath.Join(g, "wrapper", "dists", "gradle-8.5-bin", "x", "gradle.zip"))
	mkOld(t, filepath.Join(g, "caches", "7.6", "generated-gradle-jars", "a.jar"))
	mkOld(t, filepath.Join(g, "caches", "modules-2", "files-2.1", "a.jar")) // Gradle's own
	mkOld(t, filepath.Join(g, "daemon", "7.6", "daemon.log"))
	age(t, filepath.Join(g, "wrapper", "dists", "gradle-7.6-bin"), oldTime)
	age(t, filepath.Join(g, "caches", "7.6"), oldTime)
	scanClean(t, NewGradle(testEnv(home, "linux", nil, nil)), 4, 3)
	if !exists(filepath.Join(g, "caches", "modules-2")) || exists(filepath.Join(g, "caches", "7.6")) {
		t.Error("wrong Gradle caches removed")
	}
}

func TestMaven(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, ".m2", "repository")
	mkOld(t, filepath.Join(repo, "org", "x", "lib", "1.0", "lib-1.0.pom"))
	mkNew(t, filepath.Join(repo, "org", "x", "lib", "2.0", "lib-2.0.pom"))
	writeFile(t, filepath.Join(repo, "org", "x", "lib", "maven-metadata-central.xml"), "<m/>")
	scanClean(t, NewMaven(testEnv(home, "linux", nil, nil)), 2, 1)
	if exists(filepath.Join(repo, "org", "x", "lib", "1.0")) || !exists(filepath.Join(repo, "org", "x", "lib", "2.0")) {
		t.Error("wrong Maven versions removed")
	}
}

func TestPlaywrightKeepsNewest(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".cache", "ms-playwright")
	for _, d := range []string{"chromium-1100", "chromium-1140", "firefox-1400", "ffmpeg-1009"} {
		mkOld(t, filepath.Join(root, d, "bin"))
	}
	writeFile(t, filepath.Join(root, ".links", "x"), "y")
	scanClean(t, NewPlaywright(testEnv(home, "linux", nil, nil)), 4, 1)
	if exists(filepath.Join(root, "chromium-1100")) || !exists(filepath.Join(root, "chromium-1140")) || !exists(filepath.Join(root, "firefox-1400")) {
		t.Error("newest browsers must stay")
	}
	if locs := NewPlaywright(testEnv(home, "linux", map[string]string{"PLAYWRIGHT_BROWSERS_PATH": "0"}, nil)).(*dirSource).locate(context.Background()); locs != nil {
		t.Error("PLAYWRIGHT_BROWSERS_PATH=0 should mean no shared cache")
	}
	if locs := NewPlaywright(testEnv(home, "windows", nil, nil)).(*dirSource).locate(context.Background()); locs[0].root != filepath.Join(home, "AppData", "Local", "ms-playwright") {
		t.Error(locs[0].root)
	}
}

func TestCypressKeepsNewest(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "Library", "Caches", "Cypress")
	for _, d := range []string{"12.17.4", "13.6.0", "9.7.0"} {
		mkOld(t, filepath.Join(root, d, "Cypress.app"))
	}
	scanClean(t, NewCypress(testEnv(home, "darwin", nil, nil)), 3, 2)
	if !exists(filepath.Join(root, "13.6.0")) || exists(filepath.Join(root, "9.7.0")) {
		t.Error("13.6.0 must stay")
	}
	if locs := NewCypress(testEnv(home, "windows", nil, nil)).(*dirSource).locate(context.Background()); locs[0].root != filepath.Join(home, "AppData", "Local", "Cypress", "Cache") {
		t.Error(locs[0].root)
	}
}

func TestXcode(t *testing.T) {
	home := t.TempDir()
	dev := filepath.Join(home, "Library", "Developer", "Xcode")
	mkOld(t, filepath.Join(dev, "DerivedData", "App-abc", "Build", "x"))
	mkNew(t, filepath.Join(dev, "DerivedData", "Other-def", "Build", "x"))
	mkOld(t, filepath.Join(dev, "iOS DeviceSupport", "iPhone15,2 16.0 (20A362)", "Symbols"))
	mkOld(t, filepath.Join(dev, "Archives", "2025-01-01", "App.xcarchive", "Info.plist"))
	age(t, filepath.Join(dev, "DerivedData", "App-abc"), oldTime)
	scanClean(t, NewXcode(testEnv(home, "darwin", nil, nil)), 3, 2)
	if !exists(filepath.Join(dev, "Archives", "2025-01-01")) {
		t.Error("Archives must never be touched")
	}
	if locs := NewXcode(testEnv(home, "linux", nil, nil)).(*dirSource).locate(context.Background()); locs != nil {
		t.Error("Xcode is macOS only")
	}
}

func TestTargets(t *testing.T) {
	home := t.TempDir()
	rust := filepath.Join(home, "code", "rusty")
	mvn := filepath.Join(home, "code", "javay")
	other := filepath.Join(home, "code", "notes")
	writeFile(t, filepath.Join(rust, "Cargo.toml"), "[package]")
	writeFile(t, filepath.Join(rust, "target", "debug", "app"), strings.Repeat("x", 5000))
	writeFile(t, filepath.Join(mvn, "pom.xml"), "<project/>")
	writeFile(t, filepath.Join(mvn, "target", "app.jar"), "x")
	writeFile(t, filepath.Join(other, "target", "keep.txt"), "mine") // not build output
	for _, p := range []string{filepath.Join(rust, "Cargo.toml"), rust, filepath.Join(mvn, "pom.xml"), mvn} {
		age(t, p, oldTime)
	}
	s := NewTargets(testEnv(home, "linux", nil, nil))
	if s.ID() != "target" || !IsProjects("target") || IsProjects("npm") {
		t.Fatal("id")
	}
	r := scanClean(t, s, 2, 2)
	if len(r.Notes) != 0 {
		t.Errorf("notes = %v", r.Notes)
	}
	if exists(filepath.Join(rust, "target")) || exists(filepath.Join(mvn, "target")) || !exists(filepath.Join(other, "target")) {
		t.Error("wrong target folders removed")
	}
}

func TestKeepNewestAndVersions(t *testing.T) {
	cases := []struct{ name, kind, version string }{
		{"chromium-1140", "chromium", "1140"},
		{"chromium_headless_shell-1140", "chromium_headless_shell", "1140"},
		{"13.6.0", "", "13.6.0"},
		{"ModuleCache", "ModuleCache", ""},
		{"x-", "x-", ""},
	}
	for _, c := range cases {
		if k, v := splitVersion(c.name); k != c.kind || v != c.version {
			t.Errorf("splitVersion(%q) = %q, %q", c.name, k, v)
		}
	}
	if compareVersions("13.10.0", "13.9.9") <= 0 || compareVersions("1.0", "1.0.0") != 0 || compareVersions("a-1", "a-2") >= 0 {
		t.Error("compareVersions")
	}
}

func TestUV(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	dir := filepath.Join(home, "uvcache")
	mkOld(t, filepath.Join(dir, "archive-v0", "abc", "pkg.py"))
	cleaned := false
	r := &fakeRunner{installed: map[string]bool{"uv": true}, handler: func(cmd string) ([]byte, error, bool) {
		switch cmd {
		case "uv cache dir":
			return []byte(dir + "\n"), nil, true
		case "uv cache clean":
			cleaned = true
			_ = os.RemoveAll(filepath.Join(dir, "archive-v0"))
			return nil, nil, true
		}
		return nil, nil, false
	}}
	s := NewUV(testEnv(home, "linux", nil, r))
	if s.ID() != "uv" || s.Name() != "uv cache" {
		t.Fatal("id")
	}
	rep := s.Scan(ctx, month)
	if !rep.Found || rep.PrunableBytes != 0 || !hasNote(rep, "--all") {
		t.Fatalf("age mode: %+v", rep)
	}
	if res := s.Clean(ctx, rep, month); res.Removed != 0 || cleaned {
		t.Fatal("cleaned without --all")
	}
	rep = s.Scan(ctx, every)
	if rep.PrunableBytes == 0 {
		t.Fatalf("all mode: %+v", rep)
	}
	res := s.Clean(ctx, rep, every)
	if !cleaned || res.Freed == 0 || len(res.Notes) != 1 {
		t.Fatalf("clean: %+v", res)
	}

	// uv not installed: found, but never cleaned.
	none := NewUV(testEnv(home, "linux", map[string]string{"UV_CACHE_DIR": dir}, nil))
	mkOld(t, filepath.Join(dir, "x", "y"))
	if rep := none.Scan(ctx, every); !rep.Found || rep.PrunableBytes != 0 || !hasNote(rep, "isn't installed") {
		t.Fatalf("no uv: %+v", rep)
	}
	// Defaults.
	if d := NewUV(testEnv(home, "windows", nil, nil)).(*uvSource).dir(ctx); d != filepath.Join(home, "AppData", "Local", "uv", "cache") {
		t.Error(d)
	}
	if d := NewUV(testEnv(home, "darwin", nil, nil)).(*uvSource).dir(ctx); d != filepath.Join(home, ".cache", "uv") {
		t.Error(d)
	}
	if rep := NewUV(testEnv(t.TempDir(), "linux", nil, nil)).Scan(ctx, every); rep.Found {
		t.Error("missing cache found")
	}
}
