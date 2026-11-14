package appcore

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/source"
)

type fakeSource struct {
	id, name string
	report   source.Report
	result   source.Result
	cleaned  int
	opts     source.Options
}

func (f *fakeSource) ID() string   { return f.id }
func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Scan(_ context.Context, o source.Options) *source.Report {
	f.opts = o
	r := f.report
	r.ID, r.Name = f.id, f.name
	return &r
}

func (f *fakeSource) Clean(context.Context, *source.Report, source.Options) *source.Result {
	f.cleaned++
	r := f.result
	r.ID, r.Name = f.id, f.name
	return &r
}

var (
	home = func() string {
		abs, err := filepath.Abs(filepath.Join(string(filepath.Separator)+"home", "me"))
		if err != nil {
			panic(err)
		}
		return abs // has a drive letter on Windows
	}()
	now = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
)

func setup(srcs ...*fakeSource) (*Core, *time.Time, *[]string) {
	clock := now
	var revealed []string
	list := make([]source.Source, len(srcs))
	for i, s := range srcs {
		list[i] = s
	}
	c := New(Deps{
		Sources: func() ([]source.Source, string, error) { return list, home, nil },
		Busy:    func(context.Context) (map[string]string, error) { return map[string]string{}, nil },
		Now:     func() time.Time { return clock },
		Reveal:  func(p string) error { revealed = append(revealed, p); return nil },
	})
	return c, &clock, &revealed
}

func npmSrc() *fakeSource {
	return &fakeSource{id: "npm", name: "npm cache",
		report: source.Report{Found: true, Bytes: 4000, PrunableBytes: 3000, PrunableItems: 10, Items: 20,
			Locations: []string{filepath.Join(home, ".npm")}, Notes: []string{"a note"}},
		result: source.Result{Freed: 3000, Removed: 10}}
}

func dockerSrc() *fakeSource {
	return &fakeSource{id: "docker", name: "Docker",
		report: source.Report{Found: true, Bytes: 9000, PrunableBytes: 5000, Estimate: true, Locations: []string{"docker (Docker Desktop)"}},
		result: source.Result{Freed: 4500, Errors: []string{"boom"}}}
}

func TestScan(t *testing.T) {
	npm, docker := npmSrc(), dockerSrc()
	yarn := &fakeSource{id: "yarn", name: "Yarn cache"}
	c, _, _ := setup(npm, docker, yarn)
	var mu sync.Mutex
	var events []Progress
	sum, err := c.Scan(context.Background(), ScanRequest{OlderThanDays: 7, Volumes: true}, func(p Progress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if npm.opts.Age != 7*24*time.Hour || !npm.opts.Volumes || !npm.opts.Now.Equal(now) {
		t.Errorf("options = %+v", npm.opts)
	}
	if len(events) != 6 {
		t.Errorf("events = %+v", events)
	}
	if sum.Bytes != 13000 || sum.PrunableBytes != 8000 || len(sum.Sources) != 2 || sum.OlderThanDays != 7 || !sum.Volumes {
		t.Fatalf("summary = %+v", sum)
	}
	if sum.Sources[0].ID != "docker" || !sum.Sources[0].Estimate || sum.Sources[0].Locations[0].Path != "" {
		t.Errorf("docker first = %+v", sum.Sources[0])
	}
	want := "~" + string(filepath.Separator) + ".npm"
	if loc := sum.Sources[1].Locations[0]; loc.Display != want {
		t.Errorf("npm location = %+v, want %s", loc, want)
	}
	if len(sum.Missing) != 1 || sum.Missing[0] != "Yarn cache" {
		t.Errorf("missing = %v", sum.Missing)
	}
}

func TestScanErrors(t *testing.T) {
	c, _, _ := setup(npmSrc())
	if _, err := c.Scan(context.Background(), ScanRequest{OlderThanDays: -1}, nil); err == nil {
		t.Error("negative age accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Scan(ctx, ScanRequest{}, nil); err == nil {
		t.Error("cancelled scan succeeded")
	}
	bad := New(Deps{Sources: func() ([]source.Source, string, error) { return nil, "", errors.New("no home") }})
	if _, err := bad.Scan(context.Background(), ScanRequest{}, nil); err == nil {
		t.Error("sources error ignored")
	}
}

func TestClean(t *testing.T) {
	npm, docker := npmSrc(), dockerSrc()
	c, _, _ := setup(npm, docker)
	if _, err := c.Clean(context.Background(), CleanRequest{IDs: []string{"npm"}}, nil); !errors.Is(err, ErrNoScan) {
		t.Fatalf("clean without scan: %v", err)
	}
	if _, err := c.Scan(context.Background(), ScanRequest{OlderThanDays: 30}, nil); err != nil {
		t.Fatal(err)
	}
	var events []Progress
	sum, err := c.Clean(context.Background(), CleanRequest{IDs: []string{"npm", "docker"}}, func(p Progress) { events = append(events, p) })
	if err != nil {
		t.Fatal(err)
	}
	if sum.Freed != 7500 || len(sum.Results) != 2 || sum.Results[1].Errors[0] != "boom" || len(events) != 4 {
		t.Fatalf("summary = %+v, events = %v", sum, events)
	}
	// A scan is good for one clean.
	if _, err := c.Clean(context.Background(), CleanRequest{IDs: []string{"npm"}}, nil); !errors.Is(err, ErrNoScan) {
		t.Fatalf("second clean: %v", err)
	}
}

func TestCleanSelectionAndStale(t *testing.T) {
	npm, docker := npmSrc(), dockerSrc()
	c, clock, _ := setup(npm, docker)
	if _, err := c.Scan(context.Background(), ScanRequest{OlderThanDays: 30}, nil); err != nil {
		t.Fatal(err)
	}
	sum, err := c.Clean(context.Background(), CleanRequest{IDs: []string{"npm"}}, nil)
	if err != nil || len(sum.Results) != 1 || docker.cleaned != 0 {
		t.Fatalf("summary = %+v, err = %v", sum, err)
	}
	if _, err := c.Scan(context.Background(), ScanRequest{}, nil); err != nil {
		t.Fatal(err)
	}
	*clock = clock.Add(MaxScanAge + time.Second)
	if _, err := c.Clean(context.Background(), CleanRequest{IDs: []string{"npm"}}, nil); !errors.Is(err, ErrNoScan) {
		t.Fatalf("stale scan: %v", err)
	}
}

func TestCleanBusyAndForce(t *testing.T) {
	npm := npmSrc()
	c, _, _ := setup(npm)
	c.d.Busy = func(context.Context) (map[string]string, error) { return map[string]string{"npm": "npm install"}, nil }
	_, _ = c.Scan(context.Background(), ScanRequest{}, nil)
	sum, _ := c.Clean(context.Background(), CleanRequest{IDs: []string{"npm"}}, nil)
	if len(sum.Results) != 1 || sum.Results[0].Skipped == "" || npm.cleaned != 0 {
		t.Fatalf("busy: %+v", sum)
	}
	_, _ = c.Scan(context.Background(), ScanRequest{}, nil)
	sum, _ = c.Clean(context.Background(), CleanRequest{IDs: []string{"npm"}, Force: true}, nil)
	if sum.Results[0].Skipped != "" || npm.cleaned != 1 {
		t.Fatalf("force: %+v", sum)
	}
}

func TestCleanSkipsEmptyAndKeptProjects(t *testing.T) {
	empty := &fakeSource{id: "bun", name: "Bun cache", report: source.Report{Found: true, Bytes: 10}}
	nm := &fakeSource{id: "node_modules", name: "node_modules", report: source.Report{Found: true, Bytes: 10, PrunableBytes: 5, PrunableItems: 1}}
	c, _, _ := setup(empty, nm)
	_, _ = c.Scan(context.Background(), ScanRequest{}, nil)
	sum, _ := c.Clean(context.Background(), CleanRequest{IDs: []string{"bun", "node_modules"}, KeepProjects: []string{"x"}}, nil)
	if empty.cleaned != 0 || nm.cleaned != 1 || len(sum.Results) != 1 {
		t.Fatalf("summary = %+v", sum)
	}
}

func TestBusy(t *testing.T) {
	c, _, _ := setup(npmSrc())
	c.busy = true
	if _, err := c.Scan(context.Background(), ScanRequest{}, nil); !errors.Is(err, ErrBusy) {
		t.Error(err)
	}
	if _, err := c.Clean(context.Background(), CleanRequest{}, nil); !errors.Is(err, ErrBusy) {
		t.Error(err)
	}
}

func TestReveal(t *testing.T) {
	c, _, revealed := setup(npmSrc())
	p := filepath.Join(home, ".npm")
	if err := c.Reveal(p); err == nil {
		t.Error("reveal before scan allowed")
	}
	_, _ = c.Scan(context.Background(), ScanRequest{}, nil)
	if err := c.Reveal(p); err != nil || len(*revealed) != 1 {
		t.Errorf("reveal: %v %v", err, *revealed)
	}
	if err := c.Reveal(filepath.Join(home, "secret")); err == nil {
		t.Error("reveal of unknown path allowed")
	}
}

func TestTildify(t *testing.T) {
	sep := string(filepath.Separator)
	for _, tc := range []struct{ in, home, want string }{
		{home, home, "~"},
		{filepath.Join(home, "a"), home, "~" + sep + "a"},
		{sep + "tmp", home, sep + "tmp"},
		{sep + "tmp", "", sep + "tmp"},
	} {
		if got := Tildify(tc.in, tc.home); got != tc.want {
			t.Errorf("Tildify(%q, %q) = %q, want %q", tc.in, tc.home, got, tc.want)
		}
	}
}

func TestRevealCommand(t *testing.T) {
	p := filepath.Join(home, "x")
	if n, a := revealCommand("darwin", p); n != "open" || a[0] != "-R" {
		t.Error(n, a)
	}
	if n, a := revealCommand("windows", p); n != "explorer" || a[0] != "/select,"+p {
		t.Error(n, a)
	}
	if n, a := revealCommand("linux", p); n != "xdg-open" || a[0] != home {
		t.Error(n, a)
	}
}

func TestSystemDeps(t *testing.T) {
	d := SystemDeps()
	if d.Sources == nil || d.Busy == nil || d.Now == nil || d.Reveal == nil {
		t.Fatal("missing deps")
	}
	srcs, h, err := d.Sources()
	if err != nil || len(srcs) != len(source.IDs()) || h == "" {
		t.Fatalf("sources: %d %q %v", len(srcs), h, err)
	}
}

func TestViewProjects(t *testing.T) {
	v := view(&source.Report{ID: "node_modules", Name: "node_modules", Found: true, Locations: []string{home}}, home, now)
	if len(v.Projects) != 0 || v.Locations[0].Display != "~" {
		t.Errorf("view = %+v", v)
	}
}

func TestCleanLock(t *testing.T) {
	npm := npmSrc()
	c, _, _ := setup(npm)
	var lock sync.Mutex
	c.d.CleanLock = &lock
	_, _ = c.Scan(context.Background(), ScanRequest{}, nil)
	lock.Lock()
	done := make(chan struct{})
	go func() {
		_, _ = c.Clean(context.Background(), CleanRequest{IDs: []string{"npm"}}, nil)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("clean ran while the lock was held")
	case <-time.After(50 * time.Millisecond):
	}
	lock.Unlock()
	<-done
	if npm.cleaned != 1 {
		t.Fatal("clean did not run after unlock")
	}
}

func TestScanOnlyAndProjects(t *testing.T) {
	npm, docker := npmSrc(), dockerSrc()
	c, _, _ := setup(npm, docker)
	sum, err := c.Scan(context.Background(), ScanRequest{Only: []string{"npm"}, Projects: []string{home}}, nil)
	if err != nil || len(sum.Sources) != 1 || sum.Sources[0].ID != "npm" {
		t.Fatalf("summary = %+v, %v", sum, err)
	}
	if len(npm.opts.Projects) != 1 || !docker.opts.Now.IsZero() {
		t.Errorf("npm opts = %+v; docker scanned = %v", npm.opts, !docker.opts.Now.IsZero())
	}
}

type trashSrc struct{ fakeSource }

func (f *trashSrc) Clean(_ context.Context, _ *source.Report, o source.Options) *source.Result {
	f.cleaned++
	if o.Trash {
		return &source.Result{ID: f.id, Name: f.name, Trashed: 7}
	}
	return &source.Result{ID: f.id, Name: f.name, Freed: 7}
}

func TestCleanTrash(t *testing.T) {
	nm := &trashSrc{fakeSource{id: "node_modules", name: "node_modules", report: source.Report{Found: true, Bytes: 10, PrunableBytes: 7, PrunableItems: 1}}}
	c := New(Deps{
		Sources: func() ([]source.Source, string, error) { return []source.Source{nm}, home, nil },
		Busy:    func(context.Context) (map[string]string, error) { return nil, nil },
		Now:     func() time.Time { return now },
	})
	_, _ = c.Scan(context.Background(), ScanRequest{}, nil)
	sum, err := c.Clean(context.Background(), CleanRequest{IDs: []string{"node_modules"}, Trash: true}, nil)
	if err != nil || sum.Trashed != 7 || sum.Freed != 0 || sum.Results[0].Trashed != 7 {
		t.Fatalf("summary = %+v, %v", sum, err)
	}
}
