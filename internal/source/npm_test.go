package source

import (
	"context"
	"crypto/sha1" //nolint:gosec // cacache format
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeCache struct {
	t    *testing.T
	root string // _cacache
}

func integrity(content string) string {
	sum := sha512.Sum512([]byte(content))
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

// content stores data under content-v2 and returns its integrity.
func (c *fakeCache) content(data string) string {
	in := integrity(data)
	writeFile(c.t, contentPath(c.root, in), data)
	return in
}

func indexLine(key string, integ *string, when time.Time) string {
	b, _ := json.Marshal(map[string]any{"key": key, "integrity": integ, "time": when.UnixMilli(), "size": 1})
	sum := sha1.Sum(b) //nolint:gosec // cacache format
	return hex.EncodeToString(sum[:]) + "\t" + string(b)
}

func (c *fakeCache) bucket(name string, lines ...string) string {
	p := filepath.Join(c.root, "index-v5", name[:2], name[2:4], name[4:])
	writeFile(c.t, p, "\n"+strings.Join(lines, "\n"))
	return p
}

func TestContentPath(t *testing.T) {
	root := "r"
	in := integrity("x")
	p := contentPath(root, "sha1-AAAA "+in+"?opt")
	if !strings.Contains(p, filepath.Join("content-v2", "sha512")) {
		t.Errorf("strongest algorithm not chosen: %s", p)
	}
	if contentPath(root, "bogus") != "" || contentPath(root, "sha512-!!!") != "" || contentPath(root, "sha512-AA==") != "" {
		t.Error("invalid integrity should map to no path")
	}
	if contentPath(root, "weird-AAAA") != "" {
		t.Error("unknown algorithm should map to no path")
	}
}

func TestNPMAgePrune(t *testing.T) {
	ctx := context.Background()
	cacheDir := t.TempDir()
	c := &fakeCache{t: t, root: filepath.Join(cacheDir, "_cacache")}

	a := c.content("old package")
	b := c.content("new package")
	shared := c.content("shared package")
	d := c.content("deleted package")
	orphan := c.content("orphan")
	age(t, contentPath(c.root, orphan), oldTime)

	oldBucket := c.bucket("aaaaaa", indexLine("old", &a, oldTime))
	newBucket := c.bucket("bbbbbb",
		indexLine("new", &b, testNow),
		"deadbeef\t{\"key\":\"corrupt\"}",
		"no-tab-here",
	)
	sharedOld := c.bucket("cccccc", indexLine("shared-old", &shared, oldTime))
	sharedNew := c.bucket("dddddd", indexLine("shared-new", &shared, testNow))
	deleted := c.bucket("eeeeee", indexLine("deleted", &d, testNow), indexLine("deleted", nil, testNow))
	untouched := c.bucket("ffffff", indexLine("new2", &b, testNow), indexLine("new2", &b, testNow))
	tmp := filepath.Join(c.root, "tmp", "partial")
	writeFile(t, tmp, "x")
	age(t, tmp, oldTime)

	writeFile(t, filepath.Join(cacheDir, "_logs", "old.log"), "log")
	age(t, filepath.Join(cacheDir, "_logs", "old.log"), oldTime)
	writeFile(t, filepath.Join(cacheDir, "_npx", "abc", "package.json"), "{}")

	env := testEnv(t.TempDir(), "linux", map[string]string{"npm_config_cache": cacheDir}, nil)
	s := NewNPM(env)
	if s.ID() != "npm" || s.Name() != "npm cache" {
		t.Fatal("bad id/name")
	}
	r := s.Scan(ctx, month)
	if !r.Found || r.Bytes == 0 {
		t.Fatalf("report = %+v", r)
	}
	// content a, orphan, tmp, old log
	if r.PrunableItems != 4 {
		t.Errorf("PrunableItems = %d, want 4", r.PrunableItems)
	}
	if r.PrunableBytes == 0 {
		t.Error("PrunableBytes = 0")
	}

	res := s.Clean(ctx, r, month)
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	for path, want := range map[string]bool{
		contentPath(c.root, a):      false,
		contentPath(c.root, orphan): false,
		contentPath(c.root, b):      true,
		contentPath(c.root, shared): true,
		contentPath(c.root, d):      true, // no live entry; file itself is new
		oldBucket:                   false,
		sharedOld:                   false,
		deleted:                     false,
		newBucket:                   true,
		sharedNew:                   true,
		untouched:                   true,
		tmp:                         false,
		filepath.Join(cacheDir, "_logs", "old.log"): false,
		filepath.Join(cacheDir, "_npx", "abc"):      true,
	} {
		if exists(path) != want {
			t.Errorf("%s exists = %v, want %v", path, !want, want)
		}
	}
	data, _ := os.ReadFile(newBucket)
	if strings.Contains(string(data), "corrupt") || strings.Contains(string(data), "no-tab") {
		t.Errorf("corrupt lines kept: %q", data)
	}
	if !strings.HasPrefix(string(data), "\n") || !strings.Contains(string(data), `"key":"new"`) {
		t.Errorf("rewritten bucket malformed: %q", data)
	}
	for _, line := range strings.Split(strings.TrimPrefix(string(data), "\n"), "\n") {
		if _, ok := validLine(line); !ok {
			t.Errorf("invalid line after rewrite: %q", line)
		}
	}
	// Empty index folders are pruned.
	if exists(filepath.Join(c.root, "index-v5", "aa")) {
		t.Error("empty index folder left behind")
	}

	// Second scan: nothing old left.
	r2 := s.Scan(ctx, month)
	if r2.PrunableItems != 0 {
		t.Errorf("after clean, PrunableItems = %d", r2.PrunableItems)
	}
	if res := s.Clean(ctx, &Report{}, month); res.Freed != 0 {
		t.Error("clean with no plan freed space")
	}
}

func TestNPMCleanAll(t *testing.T) {
	ctx := context.Background()
	cacheDir := t.TempDir()
	c := &fakeCache{t: t, root: filepath.Join(cacheDir, "_cacache")}
	x := c.content("x")
	c.bucket("aaaaaa", indexLine("x", &x, testNow))
	env := testEnv(t.TempDir(), "linux", map[string]string{"NPM_CONFIG_CACHE": cacheDir}, nil)
	s := NewNPM(env)
	r := s.Scan(ctx, every)
	res := s.Clean(ctx, r, every)
	if res.Freed == 0 || res.Removed == 0 || len(res.Errors) > 0 {
		t.Fatalf("result = %+v", res)
	}
	if exists(contentPath(c.root, x)) {
		t.Error("content left after --all")
	}
}

func TestNPMDir(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	custom := t.TempDir()

	r := &fakeRunner{installed: map[string]bool{"npm": true}, out: map[string]string{"npm config get cache": custom + "\n"}}
	s := &npmSource{env: testEnv(home, "linux", nil, r)}
	if got := s.dir(ctx); got != custom {
		t.Errorf("dir from npm = %q", got)
	}
	r.out["npm config get cache"] = "relative/path"
	if got := s.dir(ctx); got != filepath.Join(home, ".npm") {
		t.Errorf("relative answer should fall back: %q", got)
	}
	s = &npmSource{env: testEnv(home, "windows", map[string]string{"LOCALAPPDATA": filepath.Join(home, "lad")}, nil)}
	if got := s.dir(ctx); got != filepath.Join(home, "lad", "npm-cache") {
		t.Errorf("windows default = %q", got)
	}
	s = &npmSource{env: testEnv(home, "windows", nil, nil)}
	if got := s.dir(ctx); got != filepath.Join(home, "AppData", "Local", "npm-cache") {
		t.Errorf("windows default without LOCALAPPDATA = %q", got)
	}
	// Missing cache: not found.
	if rep := NewNPM(testEnv(home, "linux", nil, nil)).Scan(ctx, month); rep.Found {
		t.Error("missing cache reported as found")
	}
}

func TestNPMNoCacache(t *testing.T) {
	cacheDir := t.TempDir()
	writeFile(t, filepath.Join(cacheDir, "_logs", "a.log"), "x")
	r := NewNPM(testEnv(t.TempDir(), "linux", map[string]string{"npm_config_cache": cacheDir}, nil)).Scan(context.Background(), every)
	if !r.Found || r.PrunableItems != 1 {
		t.Errorf("report = %+v", r)
	}
}

func TestRewriteBucketErrors(t *testing.T) {
	if err := rewriteBucket(filepath.Join(t.TempDir(), "missing-dir", "bucket"), []string{"x"}); err == nil {
		t.Error("want error for missing folder")
	}
	if runtime.GOOS == "windows" {
		return
	}
	// Renaming over a folder fails.
	dir := t.TempDir()
	target := filepath.Join(dir, "bucket")
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := rewriteBucket(target, []string{"x"}); err == nil {
		t.Error("want error renaming over a non-empty folder")
	}
}
