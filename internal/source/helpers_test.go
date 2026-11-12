package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRunner answers commands from a table or a handler.
type fakeRunner struct {
	installed map[string]bool
	out       map[string]string
	errs      map[string]error
	handler   func(cmd string) ([]byte, error, bool)
	calls     []string
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if f.installed[name] {
		return "/usr/bin/" + name, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, cmd)
	if f.handler != nil {
		if b, err, ok := f.handler(cmd); ok {
			return b, err
		}
	}
	if err, ok := f.errs[cmd]; ok {
		return nil, err
	}
	if s, ok := f.out[cmd]; ok {
		return []byte(s), nil
	}
	return nil, errors.New("unexpected command: " + cmd)
}

func testEnv(home, goos string, vars map[string]string, r *fakeRunner) Env {
	if r == nil {
		r = &fakeRunner{}
	}
	return Env{
		Home:   home,
		GOOS:   goos,
		Getenv: func(k string) string { return vars[k] },
		Run:    r,
	}
}

var (
	testNow = time.Now()
	oldTime = testNow.Add(-60 * 24 * time.Hour)
	month   = Options{Age: 30 * 24 * time.Hour, Now: testNow}
	every   = Options{Now: testNow}
)

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func age(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func hasNote(r *Report, substr string) bool {
	for _, n := range r.Notes {
		if strings.Contains(n, substr) {
			return true
		}
	}
	return false
}
