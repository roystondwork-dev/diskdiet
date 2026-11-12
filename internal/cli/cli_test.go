package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/source"
)

// fakeSource returns a fixed report and records what Clean was asked.
type fakeSource struct {
	id, name string
	report   source.Report
	result   source.Result
	gotAge   time.Duration
	cleaned  bool
}

func (f *fakeSource) ID() string   { return f.id }
func (f *fakeSource) Name() string { return f.name }

func (f *fakeSource) Scan(_ context.Context, o source.Options) *source.Report {
	f.gotAge = o.Age
	r := f.report
	r.ID, r.Name = f.id, f.name
	return &r
}

func (f *fakeSource) Clean(context.Context, *source.Report, source.Options) *source.Result {
	f.cleaned = true
	r := f.result
	r.ID, r.Name = f.id, f.name
	return &r
}

const home = "/home/u"

func fakes() []*fakeSource {
	return []*fakeSource{
		{id: "npm", name: "npm cache",
			report: source.Report{Found: true, Locations: []string{home + "/.npm"}, Bytes: 40e9, Items: 100, PrunableBytes: 30e9, PrunableItems: 80},
			result: source.Result{Freed: 30e9, Removed: 80}},
		{id: "yarn", name: "Yarn cache"},
		{id: "docker", name: "Docker",
			report: source.Report{Found: true, Locations: []string{"docker (Docker Desktop)"}, Bytes: 12e9, PrunableBytes: 8e9, Estimate: true,
				Notes: []string{"keep volumes"}},
			result: source.Result{Freed: 7e9, Notes: []string{"give it a minute"}}},
		{id: "node_modules", name: "node_modules",
			report: source.Report{Found: true, Locations: []string{home}, Bytes: 5e9, Items: 10, PrunableBytes: 2e9, PrunableItems: 3},
			result: source.Result{Freed: 2e9, Removed: 3}},
	}
}

type harness struct {
	srcs        []*fakeSource
	out, err    bytes.Buffer
	in          string
	interactive bool
	busy        map[string]string
}

func (h *harness) run(args ...string) int {
	all := make([]source.Source, len(h.srcs))
	for i, s := range h.srcs {
		all[i] = s
	}
	d := Deps{
		Sources:     func() ([]source.Source, string, error) { return all, home, nil },
		Busy:        func(context.Context) (map[string]string, error) { return h.busy, nil },
		Now:         func() time.Time { return time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC) },
		In:          strings.NewReader(h.in),
		Out:         &h.out,
		Err:         &h.err,
		Interactive: h.interactive,
	}
	return Execute(context.Background(), d, args)
}

func newHarness() *harness { return &harness{srcs: fakes()} }

func TestScanText(t *testing.T) {
	h := newHarness()
	if code := h.run(); code != ExitOK {
		t.Fatalf("exit %d: %s", code, h.err.String())
	}
	out := h.out.String()
	for _, want := range []string{"WHAT", "npm cache", "40.0 GB", "30.0 GB", "up to 8.0 GB", tildify(home+"/.npm", home),
		"2.0 GB (3 of 10)", "10 folders under ~", "keep volumes", "Not found: Yarn cache",
		"Total 57.0 GB, of which about 40.0 GB can be freed (older than 30d)", "`diskdiet clean`"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// Biggest first.
	if strings.Index(out, "npm cache") > strings.Index(out, "Docker") {
		t.Error("not sorted by size")
	}
	if h.srcs[0].gotAge != 30*24*time.Hour {
		t.Errorf("default age = %v", h.srcs[0].gotAge)
	}
}

func TestScanHints(t *testing.T) {
	h := newHarness()
	h.run("scan", "--older-than", "7d")
	if !strings.Contains(h.out.String(), "`diskdiet clean --older-than 1w`") {
		t.Errorf("hint:\n%s", h.out.String())
	}
	h = newHarness()
	h.run("scan", "--all")
	if !strings.Contains(h.out.String(), "`diskdiet clean --all`") || !strings.Contains(h.out.String(), "(in total)") {
		t.Errorf("--all hint:\n%s", h.out.String())
	}
	if h.srcs[0].gotAge != 0 {
		t.Error("--all should mean age 0")
	}
}

func TestScanNothing(t *testing.T) {
	h := &harness{srcs: []*fakeSource{{id: "npm", name: "npm cache"}}}
	h.run("scan")
	if !strings.Contains(h.out.String(), "No caches found.") {
		t.Errorf("out:\n%s", h.out.String())
	}
	h = &harness{srcs: []*fakeSource{{id: "docker", name: "Docker",
		report: source.Report{Found: true, Unavailable: true, Locations: []string{"docker"}, Error: "boom"}}}}
	h.run("scan")
	out := h.out.String()
	if !strings.Contains(out, "Nothing older than 30d to free") || !strings.Contains(out, "! Docker: boom") {
		t.Errorf("out:\n%s", out)
	}
}

func TestScanJSON(t *testing.T) {
	h := newHarness()
	if code := h.run("scan", "--json", "--only", "npm,docker"); code != ExitOK {
		t.Fatal(code, h.err.String())
	}
	var got struct {
		SchemaVersion int             `json:"schemaVersion"`
		OlderThan     string          `json:"olderThan"`
		Sources       []source.Report `json:"sources"`
		PrunableBytes int64           `json:"prunableBytes"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || got.OlderThan != "30d" || len(got.Sources) != 2 || got.PrunableBytes != 38e9 {
		t.Errorf("json = %+v", got)
	}
}

func TestSkipAndUnknown(t *testing.T) {
	h := newHarness()
	h.run("scan", "--skip", "node-modules,docker")
	if strings.Contains(h.out.String(), "node_modules") || strings.Contains(h.out.String(), "Docker") {
		t.Errorf("skipped sources shown:\n%s", h.out.String())
	}
	h = newHarness()
	if code := h.run("clean", "--only", "foo"); code != ExitUsage || !strings.Contains(h.err.String(), `unknown source "foo"`) {
		t.Errorf("exit %d err %q", code, h.err.String())
	}
	h = newHarness()
	if code := h.run("scan", "--older-than", "soon"); code != ExitUsage {
		t.Errorf("bad age exit %d", code)
	}
	h = newHarness()
	if code := h.run("scan", "--nope"); code != ExitUsage {
		t.Errorf("bad flag exit %d", code)
	}
	h = newHarness()
	if code := h.run("bogus"); code != ExitUsage {
		t.Errorf("unknown command exit %d", code)
	}
}

func TestProjectsFlag(t *testing.T) {
	dir := t.TempDir()
	h := newHarness()
	if code := h.run("scan", "--projects", dir, "--projects", "~"); code != ExitOK {
		t.Fatal(code, h.err.String())
	}
}

func TestCleanDryRun(t *testing.T) {
	h := newHarness()
	if code := h.run("clean", "--dry-run"); code != ExitOK {
		t.Fatal(code, h.err.String())
	}
	out := h.out.String()
	for _, want := range []string{"diskdiet will remove (older than 30d)", "80 cached items",
		"unused images, stopped containers, build cache", "projects you haven't touched", "Dry run: nothing was removed."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, s := range h.srcs {
		if s.cleaned {
			t.Errorf("%s cleaned in dry run", s.id)
		}
	}
	h = newHarness()
	h.run("clean", "--dry-run", "--all", "--volumes", "--only", "docker")
	if !strings.Contains(h.out.String(), "diskdiet will remove:\n") || !strings.Contains(h.out.String(), "unused volumes") {
		t.Errorf("out:\n%s", h.out.String())
	}
}

func TestCleanConfirm(t *testing.T) {
	h := newHarness()
	h.interactive, h.in = true, "y\n"
	if code := h.run("clean"); code != ExitOK {
		t.Fatal(code, h.err.String())
	}
	out := h.out.String()
	if !strings.Contains(out, "Free about 40.0 GB? [y/N]") || !strings.Contains(out, "Freed 39.0 GB.") ||
		!strings.Contains(out, "give it a minute") {
		t.Errorf("out:\n%s", out)
	}
	if !h.srcs[0].cleaned || h.srcs[1].cleaned {
		t.Error("cleaned the wrong sources")
	}

	for _, answer := range []string{"n\n", "", "maybe\n"} {
		h = newHarness()
		h.interactive, h.in = true, answer
		if code := h.run("clean"); code != ExitOK || !strings.Contains(h.out.String(), "Nothing was removed.") {
			t.Errorf("answer %q: exit %d out:\n%s", answer, code, h.out.String())
		}
		if h.srcs[0].cleaned {
			t.Errorf("answer %q cleaned", answer)
		}
	}
}

func TestCleanRefusals(t *testing.T) {
	h := newHarness()
	if code := h.run("clean"); code != ExitUsage || !strings.Contains(h.err.String(), "add --yes") {
		t.Errorf("non-interactive: exit %d err %q", code, h.err.String())
	}
	h = newHarness()
	h.interactive = true
	if code := h.run("clean", "--json"); code != ExitUsage || !strings.Contains(h.err.String(), "--json needs") {
		t.Errorf("--json: exit %d err %q", code, h.err.String())
	}
}

func TestCleanNothing(t *testing.T) {
	h := &harness{srcs: []*fakeSource{{id: "npm", name: "npm cache", report: source.Report{Found: true, Bytes: 10}}}}
	if code := h.run("clean", "--yes"); code != ExitOK || !strings.Contains(h.out.String(), "Nothing to free older than 30d.") {
		t.Errorf("exit %d out %q", code, h.out.String())
	}
	h = &harness{srcs: []*fakeSource{{id: "npm", name: "npm cache"}}}
	h.run("clean", "--yes", "--all")
	if !strings.Contains(h.out.String(), "Nothing to free.") {
		t.Errorf("out %q", h.out.String())
	}
	h = &harness{srcs: []*fakeSource{{id: "npm", name: "npm cache"}}}
	h.run("clean", "--yes", "--json")
	if !strings.Contains(h.out.String(), `"plan": []`) {
		t.Errorf("out %q", h.out.String())
	}
}

func TestCleanBusyAndPartial(t *testing.T) {
	h := newHarness()
	h.busy = map[string]string{"npm": "npm install"}
	h.srcs[2].result.Errors = []string{"e1", "e2", "e3", "e4"}
	code := h.run("clean", "--yes")
	if code != ExitPartial {
		t.Errorf("exit %d, want %d", code, ExitPartial)
	}
	out := h.out.String()
	if h.srcs[0].cleaned || !strings.Contains(out, `skipped: "npm install" is running`) {
		t.Errorf("busy npm not skipped:\n%s", out)
	}
	if !strings.Contains(out, "4 could not be removed") || !strings.Contains(out, "… and 1 more") {
		t.Errorf("errors not shown:\n%s", out)
	}
	if !strings.Contains(h.err.String(), "some items could not be removed") {
		t.Errorf("stderr %q", h.err.String())
	}

	// --force ignores busy package managers.
	h = newHarness()
	h.busy = map[string]string{"npm": "npm install"}
	h.run("clean", "--yes", "--force")
	if !h.srcs[0].cleaned {
		t.Error("--force did not clean npm")
	}
}

func TestCleanJSON(t *testing.T) {
	h := newHarness()
	if code := h.run("clean", "--yes", "--json", "--only", "npm"); code != ExitOK {
		t.Fatal(code, h.err.String())
	}
	var got cleanJSON
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.FreedBytes != 30e9 || len(got.Results) != 1 || len(got.Plan) != 1 || got.DryRun {
		t.Errorf("json = %+v", got)
	}
	h = newHarness()
	h.run("clean", "--dry-run", "--json")
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || !got.DryRun || len(got.Plan) != 3 {
		t.Errorf("dry-run json = %+v, %v", got, err)
	}
}

func TestSourcesError(t *testing.T) {
	d := Deps{
		Sources: func() ([]source.Source, string, error) { return nil, "", errors.New("no home") },
		Now:     time.Now, In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{},
	}
	if code := Execute(context.Background(), d, []string{"scan"}); code != ExitError {
		t.Errorf("exit %d", code)
	}
	if code := Execute(context.Background(), d, []string{"clean", "--yes"}); code != ExitError {
		t.Errorf("exit %d", code)
	}
}

func TestVersion(t *testing.T) {
	h := newHarness()
	if code := h.run("version"); code != ExitOK || !strings.HasPrefix(h.out.String(), "diskdiet ") {
		t.Errorf("exit %d out %q", code, h.out.String())
	}
	h = newHarness()
	h.run("version", "--json")
	var v map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &v); err != nil || v["version"] == nil {
		t.Errorf("json %q: %v", h.out.String(), err)
	}
}

func TestProgress(t *testing.T) {
	h := newHarness()
	all := make([]source.Source, len(h.srcs))
	for i, s := range h.srcs {
		all[i] = s
	}
	d := Deps{
		Sources:  func() ([]source.Source, string, error) { return all, home, nil },
		Busy:     func(context.Context) (map[string]string, error) { return nil, errors.New("x") },
		Now:      time.Now,
		In:       strings.NewReader(""),
		Out:      &h.out,
		Err:      &h.err,
		Progress: true,
	}
	Execute(context.Background(), d, []string{"clean", "--yes"})
	if !strings.Contains(h.err.String(), "Scanning npm cache") || !strings.Contains(h.err.String(), "Cleaning npm cache") {
		t.Errorf("stderr %q", h.err.String())
	}
}

func TestHelpers(t *testing.T) {
	day := 24 * time.Hour
	for d, want := range map[time.Duration]string{
		time.Hour: "less than a day", day: "1 day", 5 * day: "5 days", 90 * day: "3 months", 400 * day: "1.1 years",
	} {
		if got := idle(d); got != want {
			t.Errorf("idle(%v) = %q, want %q", d, got, want)
		}
	}
	h := filepath.Join(string(filepath.Separator)+"home", "u")
	for in, want := range map[string]string{
		h:                                  "~",
		filepath.Join(h, "a"):              "~" + string(filepath.Separator) + "a",
		string(filepath.Separator) + "etc": string(filepath.Separator) + "etc",
	} {
		if got := tildify(in, h); got != want {
			t.Errorf("tildify(%q) = %q, want %q", in, got, want)
		}
	}
	if tildify("/x", "") != "/x" {
		t.Error("tildify without home")
	}
	if p, err := absPath("~"); err != nil || !filepath.IsAbs(p) {
		t.Errorf("absPath(~) = %q, %v", p, err)
	}
	if home, _ := os.UserHomeDir(); home != "" {
		if p, _ := absPath("~/code"); p != filepath.Join(home, "code") {
			t.Errorf("absPath(~/code) = %q", p)
		}
	}
	if planDetail(&source.Report{ID: "pnpm"}, source.Options{}) != "packages no project uses" {
		t.Error("pnpm detail")
	}
	if prunableLabel(&source.Report{}) != "–" {
		t.Error("empty prunable label")
	}
}

func TestSystemDeps(t *testing.T) {
	d := SystemDeps()
	if d.Sources == nil || d.Busy == nil || d.Out == nil {
		t.Fatal("SystemDeps incomplete")
	}
	srcs, home, err := d.Sources()
	if err != nil || len(srcs) != len(source.IDs()) || home == "" {
		t.Errorf("Sources = %d, %q, %v", len(srcs), home, err)
	}
}

func TestMCP(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	srcs := fakes()
	d := Deps{
		Sources: func() ([]source.Source, string, error) {
			list := make([]source.Source, len(srcs))
			for i, s := range srcs {
				list[i] = s
			}
			return list, "/home/me", nil
		},
		Busy: func(context.Context) (map[string]string, error) { return nil, nil },
		Now:  time.Now,
		In:   inR, Out: outW, Err: io.Discard,
	}
	done := make(chan int, 1)
	go func() { done <- Execute(context.Background(), d, []string{"mcp"}) }()
	lines := bufio.NewScanner(outR)
	send := func(msg string) {
		if _, err := io.WriteString(inW, msg+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	if !lines.Scan() || !strings.Contains(lines.Text(), `"name":"diskdiet"`) {
		t.Fatalf("initialize: %s", lines.Text())
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if !lines.Scan() || !strings.Contains(lines.Text(), `"scan_disk_usage"`) || !strings.Contains(lines.Text(), `"free_disk_space"`) {
		t.Fatalf("tools/list: %s", lines.Text())
	}
	_ = inW.Close()
	go func() { _, _ = io.Copy(io.Discard, outR) }()
	if code := <-done; code != ExitOK {
		t.Fatalf("exit %d", code)
	}
}
