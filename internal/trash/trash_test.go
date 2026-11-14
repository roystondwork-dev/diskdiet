package trash

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestFreedesktop(t *testing.T) {
	dir := t.TempDir()
	trashDir := filepath.Join(dir, "Trash")
	now := time.Date(2026, 11, 14, 10, 30, 0, 0, time.Local)
	mk := func(name string) string {
		p := filepath.Join(dir, "proj", name)
		if err := os.MkdirAll(filepath.Join(p, "dep"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "dep", "index.js"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	first := mk("node_modules")
	if err := freedesktop(trashDir, first, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("folder still in place")
	}
	if _, err := os.Stat(filepath.Join(trashDir, "files", "node_modules", "dep", "index.js")); err != nil {
		t.Fatalf("not in Trash/files: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(trashDir, "info", "node_modules.trashinfo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(info), "[Trash Info]\nPath=") || !strings.Contains(string(info), "DeletionDate=2026-11-14T10:30:00") {
		t.Fatalf("trashinfo = %q", info)
	}

	// A second folder with the same name gets a new name, not a clash.
	second := mk("node_modules")
	if err := freedesktop(trashDir, second, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(trashDir, "files", "node_modules.1")); err != nil {
		t.Fatalf("second item: %v", err)
	}

	// Moving something that isn't there fails and leaves no stray info file.
	if err := freedesktop(trashDir, filepath.Join(dir, "missing"), now); err == nil {
		t.Fatal("missing path moved")
	}
	if _, err := os.Stat(filepath.Join(trashDir, "info", "missing.trashinfo")); !os.IsNotExist(err) {
		t.Error("stray .trashinfo left behind")
	}
}

func TestFreedesktopUnusableTrash(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "Trash")
	if err := os.WriteFile(blocker, []byte("a file, not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := freedesktop(blocker, filepath.Join(dir, "x"), time.Now()); err == nil {
		t.Fatal("a Trash that can't be created was accepted")
	}
}

func TestPathEscapingInTrashInfo(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "my project", "node_modules")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := freedesktop(filepath.Join(dir, "Trash"), p, time.Now()); err != nil {
		t.Fatal(err)
	}
	info, _ := os.ReadFile(filepath.Join(dir, "Trash", "info", "node_modules.trashinfo"))
	if !strings.Contains(string(info), "my%20project") {
		t.Errorf("path not URL-escaped: %q", info)
	}
}

func TestPSQuoteAndEncode(t *testing.T) {
	if got := psQuote(`C:\a b\it's`); got != `'C:\a b\it''s'` {
		t.Errorf("psQuote = %s", got)
	}
	raw, err := base64.StdEncoding.DecodeString(encodeCommand("$p = 'é'"))
	if err != nil {
		t.Fatal(err)
	}
	u := make([]uint16, len(raw)/2)
	for i := range u {
		u[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
	}
	if got := string(utf16.Decode(u)); got != "$p = 'é'" {
		t.Errorf("round trip = %q", got)
	}
}

func stubRun(t *testing.T, out string, err error) *[][]string {
	t.Helper()
	var calls [][]string
	old := runFunc
	runFunc = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte(out), err
	}
	t.Cleanup(func() { runFunc = old })
	return &calls
}

func TestMacOSAndWindowsCommands(t *testing.T) {
	ctx := context.Background()
	calls := stubRun(t, "ok\n", nil)
	if err := macOS(ctx, "/Users/me/p/node_modules"); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c[0] != "osascript" || c[len(c)-1] != "/Users/me/p/node_modules" || !strings.Contains(c[4], "trashItemAtURL") {
		t.Errorf("osascript call = %v", c)
	}
	if err := windows(ctx, `C:\p\node_modules`); err != nil {
		t.Fatal(err)
	}
	w := (*calls)[1]
	if w[0] != "powershell" || w[len(w)-2] != "-EncodedCommand" {
		t.Errorf("powershell call = %v", w)
	}

	stubRun(t, "failed", nil)
	if err := macOS(ctx, "/x"); err == nil {
		t.Error("macOS refusing must be an error")
	}
	stubRun(t, "", errors.New("boom"))
	if macOS(ctx, "/x") == nil || windows(ctx, `C:\x`) == nil {
		t.Error("tool failures must be errors")
	}
}

func TestMoveUsesTheRunningOS(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("the Linux branch moves real files; covered by TestMoveOnEveryOS")
	}
	calls := stubRun(t, "ok", nil)
	if err := Move(context.Background(), "relative/node_modules"); err != nil || len(*calls) != 1 {
		t.Fatalf("Move: %v, calls %v", err, *calls)
	}
}

func TestMoveOnEveryOS(t *testing.T) {
	ctx := context.Background()
	calls := stubRun(t, "ok", nil)
	for _, goos := range []string{"darwin", "windows"} {
		if err := moveOn(ctx, goos, "p", nil, nil); err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
	}
	if len(*calls) != 2 || (*calls)[0][0] != "osascript" || (*calls)[1][0] != "powershell" {
		t.Fatalf("calls = %v", *calls)
	}
	// Linux: XDG_DATA_HOME, else ~/.local/share.
	dir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	env := func(v string) func(string) string { return func(string) string { return v } }
	if err := moveOn(ctx, "linux", mk("a"), nil, env(filepath.Join(dir, "xdg"))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "xdg", "Trash", "files", "a")); err != nil {
		t.Fatalf("XDG_DATA_HOME Trash: %v", err)
	}
	home := func() (string, error) { return filepath.Join(dir, "home"), nil }
	if err := moveOn(ctx, "linux", mk("b"), home, env("")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "home", ".local", "share", "Trash", "files", "b")); err != nil {
		t.Fatalf("home Trash: %v", err)
	}
	noHome := func() (string, error) { return "", errors.New("no home") }
	if err := moveOn(ctx, "linux", mk("c"), noHome, env("")); err == nil {
		t.Error("no home folder must be an error")
	}
	if err := moveOn(ctx, "plan9", "x", nil, nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("plan9: %v", err)
	}
}
