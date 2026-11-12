package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roystondwork-dev/diskdiet/internal/appcore"
	"github.com/roystondwork-dev/diskdiet/internal/source"
)

type fakeSource struct {
	id, name string
	report   source.Report
	cleaned  int
}

func (f *fakeSource) ID() string   { return f.id }
func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Scan(context.Context, source.Options) *source.Report {
	r := f.report
	r.ID, r.Name = f.id, f.name
	return &r
}

func (f *fakeSource) Clean(context.Context, *source.Report, source.Options) *source.Result {
	f.cleaned++
	return &source.Result{ID: f.id, Name: f.name, Freed: f.report.PrunableBytes}
}

const gb = int64(1e9)

type env struct {
	npm, docker *fakeSource
	clock       *time.Time
	session     *mcp.ClientSession
	asked       *[]string
}

// connect starts the server with fake sources and a client. answer, when not
// empty, makes the client support confirmation and answer with it.
func connect(t *testing.T, answer string) env {
	t.Helper()
	npm := &fakeSource{id: "npm", name: "npm cache", report: source.Report{Found: true, Bytes: 40 * gb, PrunableBytes: 30 * gb, PrunableItems: 9,
		Locations: []string{filepath.Join(string(filepath.Separator)+"h", ".npm")}}}
	docker := &fakeSource{id: "docker", name: "Docker", report: source.Report{Found: true, Bytes: 9 * gb, PrunableBytes: 5 * gb, Estimate: true,
		Locations: []string{"docker (Docker Desktop)"}, Notes: []string{"1 GB in unused volumes is kept"}}}
	missing := &fakeSource{id: "bun", name: "Bun cache"}
	clock := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	core := appcore.New(appcore.Deps{
		Sources: func() ([]source.Source, string, error) {
			return []source.Source{npm, docker, missing}, "", nil
		},
		Busy: func(context.Context) (map[string]string, error) { return nil, nil },
		Now:  func() time.Time { return clock },
	})
	server := New("1.2.3", core)
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	var asked []string
	opts := &mcp.ClientOptions{}
	if answer != "" {
		opts.ElicitationHandler = func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			asked = append(asked, req.Params.Message)
			if answer == "error" {
				return nil, errors.New("client broke")
			}
			return &mcp.ElicitResult{Action: answer}, nil
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, opts)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return env{npm: npm, docker: docker, clock: &clock, session: cs, asked: &asked}
}

func call[T any](t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (T, *mcp.CallToolResult) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if !res.IsError {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
	}
	return out, res
}

func errText(res *mcp.CallToolResult) string {
	if !res.IsError || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func TestTools(t *testing.T) {
	e := connect(t, "")
	res, err := e.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		names[tool.Name] = tool
	}
	scan, free := names["scan_disk_usage"], names["free_disk_space"]
	if scan == nil || free == nil || len(names) != 2 {
		t.Fatalf("tools = %v", names)
	}
	if !scan.Annotations.ReadOnlyHint || *free.Annotations.DestructiveHint != true {
		t.Error("annotations")
	}
	if init := e.session.InitializeResult(); init.ServerInfo.Version != "1.2.3" || !strings.Contains(init.Instructions, "free_disk_space") {
		t.Errorf("init = %+v", init)
	}
}

func TestScan(t *testing.T) {
	e := connect(t, "")
	out, res := call[ScanOutput](t, e.session, "scan_disk_usage", nil)
	if res.IsError {
		t.Fatal(errText(res))
	}
	if out.OlderThanDays != 30 || out.TotalBytes != 49*gb || out.CanFree != "35.0 GB" || len(out.Sources) != 2 {
		t.Fatalf("out = %+v", out)
	}
	if out.Sources[0].ID != "npm" || out.Sources[0].Removes == "" || out.Sources[1].Locations[0] != "docker (Docker Desktop)" || !out.Sources[1].Estimate {
		t.Errorf("sources = %+v", out.Sources)
	}
	if len(out.NotFound) != 1 || !strings.Contains(out.Summary, "35.0 GB unused for 30 days") || !strings.Contains(out.Next, "free_disk_space") {
		t.Errorf("out = %+v", out)
	}
	if out.Sources[1].Removes != describe("docker", false) {
		t.Error(out.Sources[1].Removes)
	}

	out, _ = call[ScanOutput](t, e.session, "scan_disk_usage", map[string]any{"older_than_days": 0, "sources": []string{"npm"}, "include_docker_volumes": true})
	if len(out.Sources) != 1 || !strings.Contains(out.Summary, "in total") {
		t.Errorf("filtered = %+v", out)
	}
	for _, bad := range []map[string]any{{"older_than_days": -1}, {"sources": []string{"nope"}}} {
		if _, res := call[ScanOutput](t, e.session, "scan_disk_usage", bad); !res.IsError {
			t.Errorf("%v accepted", bad)
		}
	}
}

func TestFreeWithoutConfirmationSupport(t *testing.T) {
	e := connect(t, "")
	if _, res := call[FreeOutput](t, e.session, "free_disk_space", map[string]any{"sources": []string{"npm"}}); !strings.Contains(errText(res), "scan_disk_usage first") {
		t.Fatalf("free before scan: %v", errText(res))
	}
	call[ScanOutput](t, e.session, "scan_disk_usage", nil)
	out, res := call[FreeOutput](t, e.session, "free_disk_space", map[string]any{"sources": []string{"npm"}})
	if res.IsError || out.FreedBytes != 30*gb || len(out.Results) != 1 || e.npm.cleaned != 1 || e.docker.cleaned != 0 {
		t.Fatalf("out = %+v (%s)", out, errText(res))
	}
	if out.Summary != "Freed 30.0 GB." {
		t.Error(out.Summary)
	}
	// One scan, one clean.
	if _, res := call[FreeOutput](t, e.session, "free_disk_space", map[string]any{"sources": []string{"npm"}}); !res.IsError {
		t.Error("second free without a new scan succeeded")
	}
}

func TestFreeAsksTheUser(t *testing.T) {
	e := connect(t, "accept")
	call[ScanOutput](t, e.session, "scan_disk_usage", nil)
	out, res := call[FreeOutput](t, e.session, "free_disk_space", map[string]any{"sources": []string{"npm", "docker"}})
	if res.IsError || e.npm.cleaned != 1 || e.docker.cleaned != 1 {
		t.Fatalf("out = %+v (%s)", out, errText(res))
	}
	if len(*e.asked) != 1 || !strings.Contains((*e.asked)[0], "Free about 35.0 GB from npm cache, Docker?") {
		t.Fatalf("asked = %v", *e.asked)
	}
}

func TestFreeDeclined(t *testing.T) {
	for _, answer := range []string{"decline", "cancel"} {
		e := connect(t, answer)
		call[ScanOutput](t, e.session, "scan_disk_usage", nil)
		out, res := call[FreeOutput](t, e.session, "free_disk_space", map[string]any{"sources": []string{"npm"}})
		if res.IsError || !out.Declined || e.npm.cleaned != 0 {
			t.Fatalf("%s: out = %+v", answer, out)
		}
	}
	e := connect(t, "error")
	call[ScanOutput](t, e.session, "scan_disk_usage", nil)
	res, err := e.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "free_disk_space", Arguments: map[string]any{"sources": []string{"npm"}}})
	if (err == nil && !res.IsError) || e.npm.cleaned != 0 {
		t.Fatal("a failed confirmation must not clean")
	}
}

func TestFreeValidation(t *testing.T) {
	e := connect(t, "")
	call[ScanOutput](t, e.session, "scan_disk_usage", nil)
	for _, args := range []map[string]any{{"sources": []string{}}, {"sources": []string{"nope"}}, {}} {
		if _, res := call[FreeOutput](t, e.session, "free_disk_space", args); !res.IsError {
			t.Errorf("%v accepted", args)
		}
	}
	out, res := call[FreeOutput](t, e.session, "free_disk_space", map[string]any{"sources": []string{"bun"}})
	if res.IsError || out.Summary != "Nothing to free in those sources." {
		t.Errorf("nothing to free: %+v %s", out, errText(res))
	}
}

func TestFreeStaleScan(t *testing.T) {
	e := connect(t, "")
	call[ScanOutput](t, e.session, "scan_disk_usage", nil)
	*e.clock = e.clock.Add(appcore.MaxScanAge + time.Minute)
	if _, res := call[FreeOutput](t, e.session, "free_disk_space", map[string]any{"sources": []string{"npm"}}); !strings.Contains(errText(res), "older than 15 minutes") || e.npm.cleaned != 0 {
		t.Fatalf("stale: %s", errText(res))
	}
}

func TestDescribe(t *testing.T) {
	for _, id := range source.IDs() {
		if describe(id, false) == "" {
			t.Errorf("no description for %s", id)
		}
	}
	if describe("docker", true) == describe("docker", false) || describe("x", false) != "" {
		t.Error("docker volumes / unknown")
	}
	if firstNonEmpty("", "b") != "b" || firstNonEmpty("a", "b") != "a" {
		t.Error("firstNonEmpty")
	}
}
