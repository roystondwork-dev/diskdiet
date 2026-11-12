package fsx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func write(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a"), 10_000)
	write(t, filepath.Join(dir, "sub", "b"), 20_000)
	u, err := DirSize(context.Background(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if u.Files != 2 {
		t.Errorf("Files = %d, want 2", u.Files)
	}
	if u.Bytes < 30_000 || u.Exclusive != u.Bytes {
		t.Errorf("usage = %+v", u)
	}

	var sum Usage
	sum.Add(u)
	sum.Add(u)
	if sum.Files != 4 || sum.Bytes != 2*u.Bytes || sum.Exclusive != 2*u.Exclusive {
		t.Errorf("Add = %+v", sum)
	}

	if _, err := DirSize(context.Background(), filepath.Join(dir, "missing"), Options{}); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing dir err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DirSize(ctx, dir, Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled err = %v", err)
	}
}

func TestDirSizeHardLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("link counts are read per file on Windows; covered on unix")
	}
	store := t.TempDir()
	proj := t.TempDir()
	write(t, filepath.Join(store, "shared"), 50_000)
	write(t, filepath.Join(store, "own"), 50_000)
	if err := os.Link(filepath.Join(store, "shared"), filepath.Join(proj, "shared")); err != nil {
		t.Skip("hard links not supported:", err)
	}
	// A second link inside the same tree is counted once.
	if err := os.Link(filepath.Join(store, "shared"), filepath.Join(store, "shared2")); err != nil {
		t.Fatal(err)
	}
	all, _ := DirSize(context.Background(), store, Options{})
	linked, _ := DirSize(context.Background(), store, Options{CountLinks: true})
	if all.Files != 2 {
		t.Errorf("Files = %d, want 2 (hard link counted once)", all.Files)
	}
	if linked.Exclusive >= linked.Bytes || linked.Exclusive == 0 {
		t.Errorf("exclusive %d should be less than bytes %d and not zero", linked.Exclusive, linked.Bytes)
	}
	if all.Exclusive != all.Bytes {
		t.Errorf("without CountLinks, exclusive should equal bytes: %+v", all)
	}
}

func TestChildrenAndFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "pkg-a", "index.js"), 3000)
	write(t, filepath.Join(dir, "pkg-b", "lib", "x.js"), 3000)
	write(t, filepath.Join(dir, ".tmp", "junk"), 3000)
	write(t, filepath.Join(dir, "archive.zip"), 3000)
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "pkg-a"), old, old); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	es, err := Children(ctx, dir, Options{}, func(n string) bool { return n == ".tmp" })
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 3 {
		t.Fatalf("got %d children, want 3: %+v", len(es), es)
	}
	for _, e := range es {
		if e.Usage.Bytes == 0 {
			t.Errorf("%s has no size", e.Path)
		}
		if filepath.Base(e.Path) == "pkg-a" && !e.Modified.Equal(old) {
			t.Errorf("pkg-a modified = %v, want %v", e.Modified, old)
		}
	}
	if _, err := Children(ctx, filepath.Join(dir, "nope"), Options{}, nil); err == nil {
		t.Error("Children of missing dir: want error")
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Children(cctx, dir, Options{}, nil); err == nil {
		t.Error("canceled Children: want error")
	}

	fs, err := Files(ctx, dir, Options{CountLinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 4 {
		t.Errorf("Files = %d entries, want 4", len(fs))
	}
	for _, f := range fs {
		if f.Usage.Files != 1 {
			t.Errorf("%s: Files = %d", f.Path, f.Usage.Files)
		}
	}
	if _, err := Files(ctx, filepath.Join(dir, "nope"), Options{}); err == nil {
		t.Error("Files of missing dir: want error")
	}
	if _, err := Files(cctx, dir, Options{}); err == nil {
		t.Error("canceled Files: want error")
	}
}

func TestFilesHardLinked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix link counts")
	}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a"), 100)
	if err := os.Link(filepath.Join(dir, "a"), filepath.Join(dir, "b")); err != nil {
		t.Skip(err)
	}
	fs, _ := Files(context.Background(), dir, Options{CountLinks: true})
	for _, f := range fs {
		if f.Usage.Exclusive != 0 {
			t.Errorf("%s exclusive = %d, want 0", f.Path, f.Usage.Exclusive)
		}
	}
}

func TestWithin(t *testing.T) {
	root := filepath.Join(string(filepath.Separator)+"cache", "npm")
	cases := []struct {
		path string
		want bool
	}{
		{root, true},
		{filepath.Join(root, "a", "b"), true},
		{filepath.Join(root, "..", "other"), false},
		{filepath.Join(root, "..", "npm-evil"), false},
		{filepath.Join(root, "..dots"), true},
		{string(filepath.Separator) + "elsewhere", false},
	}
	for _, c := range cases {
		if got := Within(root, c.path); got != c.want {
			t.Errorf("Within(%q, %q) = %v, want %v", root, c.path, got, c.want)
		}
	}
}

func TestRemove(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "pkg")
	write(t, filepath.Join(target, "f"), 10)
	if err := Remove(root, target); err != nil {
		t.Fatal(err)
	}
	if Exists(target) {
		t.Error("target still exists")
	}

	// Refusals.
	if err := Remove(root, root); !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("removing the root itself: %v", err)
	}
	outside := t.TempDir()
	if err := Remove(root, outside); !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("outside root: %v", err)
	}
	if err := Remove("relative", filepath.Join("relative", "x")); err == nil {
		t.Error("relative root accepted")
	}
	if err := Remove("", "x"); err == nil {
		t.Error("empty root accepted")
	}
	fsRoot := filepath.VolumeName(root) + string(filepath.Separator)
	if err := Remove(fsRoot, filepath.Join(fsRoot, "tmp", "x")); err == nil {
		t.Error("filesystem root accepted")
	}
	if home, err := os.UserHomeDir(); err == nil {
		if err := Remove(home, filepath.Join(home, "diskdiet-never-exists")); err == nil {
			t.Error("home folder accepted as root")
		}
	}
	// Missing targets are not an error.
	if err := Remove(root, filepath.Join(root, "gone")); err != nil {
		t.Errorf("missing target: %v", err)
	}
}

func TestRemoveReadOnly(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "pkg")
	write(t, filepath.Join(target, "sub", "f"), 10)
	if err := os.Chmod(filepath.Join(target, "sub", "f"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(target, "sub"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(target, "sub"), 0o755) })
	if err := Remove(root, target); err != nil {
		t.Fatal(err)
	}
	if Exists(target) {
		t.Error("read-only tree not removed")
	}
}

func TestExistsIsDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "f")
	write(t, f, 1)
	if !Exists(f) || IsDir(f) || !IsDir(dir) || Exists(filepath.Join(dir, "x")) {
		t.Error("Exists/IsDir wrong")
	}
}
