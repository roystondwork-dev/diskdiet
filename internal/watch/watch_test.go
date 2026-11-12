package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/appcore"
	"github.com/roystondwork-dev/diskdiet/internal/source"
)

type fakeSource struct {
	id      string
	report  source.Report
	freed   int64
	cleaned int
	ages    []time.Duration
}

func (f *fakeSource) ID() string   { return f.id }
func (f *fakeSource) Name() string { return f.id }
func (f *fakeSource) Scan(_ context.Context, o source.Options) *source.Report {
	f.ages = append(f.ages, o.Age)
	r := f.report
	r.ID, r.Name, r.Found = f.id, f.id, true
	return &r
}

func (f *fakeSource) Clean(context.Context, *source.Report, source.Options) *source.Result {
	f.cleaned++
	return &source.Result{ID: f.id, Name: f.id, Freed: f.freed}
}

const gb = int64(1e9)

var t0 = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

type harness struct {
	w        *Watcher
	clock    *time.Time
	notes    *[]string
	statuses *[]Status
	store    *Store
}

func newHarness(t *testing.T, srcs ...*fakeSource) harness {
	t.Helper()
	clock := t0
	list := make([]source.Source, len(srcs))
	for i, s := range srcs {
		list[i] = s
	}
	core := appcore.New(appcore.Deps{
		Sources: func() ([]source.Source, string, error) { return list, "", nil },
		Busy:    func(context.Context) (map[string]string, error) { return nil, nil },
		Now:     func() time.Time { return clock },
	})
	var mu sync.Mutex
	var notes []string
	var statuses []Status
	store := NewStore(filepath.Join(t.TempDir(), "diskdiet", "settings.json"))
	w := New(Deps{
		Core:  core,
		Store: store,
		Now:   func() time.Time { return clock },
		Notify: func(title, body string) error {
			mu.Lock()
			defer mu.Unlock()
			notes = append(notes, title+" | "+body)
			return nil
		},
		OnStatus: func(s Status) { mu.Lock(); statuses = append(statuses, s); mu.Unlock() },
	})
	return harness{w: w, clock: &clock, notes: &notes, statuses: &statuses, store: store}
}

func npm(prunable int64) *fakeSource {
	return &fakeSource{id: "npm", report: source.Report{Bytes: 3 * prunable, PrunableBytes: prunable, PrunableItems: 1}, freed: prunable}
}

func TestDefaultsAndNormalize(t *testing.T) {
	d := Defaults()
	if d.CheckEveryHours != 6 || d.WarnAboveGB != 10 || d.OlderThanDays != 30 || d.AutoClean != AutoOff {
		t.Fatalf("defaults = %+v", d)
	}
	n := Settings{CheckEveryHours: 0, WarnAboveGB: -1, OlderThanDays: 0, AutoClean: "hourly"}.normalize()
	if n.CheckEveryHours != 6 || n.WarnAboveGB != 10 || n.OlderThanDays != 30 || n.AutoClean != AutoOff {
		t.Fatalf("normalized = %+v", n)
	}
	if k := (Settings{CheckEveryHours: 12, WarnAboveGB: 0, OlderThanDays: 7, AutoClean: AutoDaily}).normalize(); k.WarnAboveGB != 0 || k.AutoClean != AutoDaily || k.CheckEveryHours != 12 {
		t.Fatalf("valid values changed: %+v", k)
	}
}

func TestStore(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "sub", "settings.json"))
	got, err := s.Load()
	if err != nil || got != Defaults() {
		t.Fatalf("missing file: %+v %v", got, err)
	}
	want := Defaults()
	want.WarnAboveGB = 20
	want.LastCheck = t0
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil || !got.LastCheck.Equal(t0) || got.WarnAboveGB != 20 {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "settings.json"), []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Load(); err == nil || got != Defaults() {
		t.Fatalf("broken file: %+v %v", got, err)
	}
	if p, err := DefaultPath(); err != nil || !strings.HasSuffix(p, filepath.Join("diskdiet", "settings.json")) {
		t.Fatalf("DefaultPath = %q, %v", p, err)
	}
	// A folder where the file should be makes reading and saving fail.
	bad := NewStore(dir)
	if _, err := bad.Load(); err == nil {
		t.Error("reading a folder succeeded")
	}
	if err := bad.Save(Defaults()); err == nil {
		t.Error("saving over a folder succeeded")
	}
}

func TestShouldNotify(t *testing.T) {
	s := Defaults() // 10 GB
	if ShouldNotify(s, 9*gb, t0) {
		t.Error("below threshold")
	}
	if !ShouldNotify(s, 10*gb, t0) {
		t.Error("first time above threshold")
	}
	s.LastNotified, s.LastNotifiedBytes = t0, 12*gb
	if ShouldNotify(s, 13*gb, t0.Add(time.Hour)) {
		t.Error("small growth soon after")
	}
	if !ShouldNotify(s, 17*gb, t0.Add(time.Hour)) {
		t.Error("grew by half the threshold")
	}
	if !ShouldNotify(s, 12*gb, t0.Add(7*24*time.Hour)) {
		t.Error("weekly reminder")
	}
	s.WarnAboveGB = 0
	if ShouldNotify(s, 100*gb, t0) {
		t.Error("warnings off")
	}
}

func TestAutoCleanDue(t *testing.T) {
	s := Defaults()
	if AutoCleanDue(s, t0) {
		t.Error("off")
	}
	s.AutoClean = AutoWeekly
	if !AutoCleanDue(s, t0) {
		t.Error("never ran")
	}
	s.LastAutoClean = t0
	if AutoCleanDue(s, t0.Add(6*24*time.Hour)) || !AutoCleanDue(s, t0.Add(7*24*time.Hour)) {
		t.Error("weekly timing")
	}
	s.AutoClean = AutoDaily
	if AutoCleanDue(s, t0.Add(23*time.Hour)) || !AutoCleanDue(s, t0.Add(24*time.Hour)) {
		t.Error("daily timing")
	}
}

func TestCheckNotifies(t *testing.T) {
	src := npm(15 * gb)
	h := newHarness(t, src)
	h.w.Check(context.Background())
	if len(*h.notes) != 1 || !strings.Contains((*h.notes)[0], "15.0 GB of developer caches can be freed") {
		t.Fatalf("notes = %v", *h.notes)
	}
	if src.ages[0] != 30*24*time.Hour || src.cleaned != 0 {
		t.Fatalf("ages = %v, cleaned = %d", src.ages, src.cleaned)
	}
	st := h.w.Status()
	if st.Checking || st.PrunableBytes != 15*gb || !st.Checked.Equal(t0) {
		t.Fatalf("status = %+v", st)
	}
	if len(*h.statuses) < 2 || !(*h.statuses)[0].Checking {
		t.Fatalf("statuses = %+v", *h.statuses)
	}
	// Saved, so a restart doesn't notify again.
	saved, _ := h.store.Load()
	if !saved.LastNotified.Equal(t0) || saved.LastNotifiedBytes != 15*gb {
		t.Fatalf("saved = %+v", saved)
	}
	*h.clock = t0.Add(time.Hour)
	h.w.Check(context.Background())
	if len(*h.notes) != 1 {
		t.Fatalf("notified twice: %v", *h.notes)
	}
}

func TestCheckAutoClean(t *testing.T) {
	safe := npm(4 * gb)
	nm := &fakeSource{id: "node_modules", report: source.Report{Bytes: 9 * gb, PrunableBytes: 8 * gb, PrunableItems: 3}}
	h := newHarness(t, safe, nm)
	if err := h.w.Update(func(s *Settings) { s.AutoClean = AutoWeekly; s.WarnAboveGB = 5 }); err != nil {
		t.Fatal(err)
	}
	h.w.Check(context.Background())
	if safe.cleaned != 1 || nm.cleaned != 0 {
		t.Fatalf("cleaned npm %d, node_modules %d", safe.cleaned, nm.cleaned)
	}
	st := h.w.Status()
	if st.LastFreed != 4*gb || st.PrunableBytes != 8*gb {
		t.Fatalf("status = %+v", st)
	}
	if len(*h.notes) != 2 || !strings.HasPrefix((*h.notes)[0], "diskdiet freed 4.0 GB") || !strings.Contains((*h.notes)[1], "8.0 GB") {
		t.Fatalf("notes = %v", *h.notes)
	}
	if !h.w.Settings().LastAutoClean.Equal(t0) {
		t.Fatal("LastAutoClean not recorded")
	}
	// Not due again the next day.
	*h.clock = t0.Add(24 * time.Hour)
	h.w.Check(context.Background())
	if safe.cleaned != 1 {
		t.Fatal("cleaned again too soon")
	}
}

func TestCheckAutoCleanNothingToDo(t *testing.T) {
	src := &fakeSource{id: "npm", report: source.Report{Bytes: gb}}
	h := newHarness(t, src)
	_ = h.w.Update(func(s *Settings) { s.AutoClean = AutoDaily })
	h.w.Check(context.Background())
	if src.cleaned != 0 || len(*h.notes) != 0 || !h.w.Settings().LastAutoClean.Equal(t0) {
		t.Fatalf("cleaned %d, notes %v", src.cleaned, *h.notes)
	}
}

func TestCheckScanError(t *testing.T) {
	h := newHarness(t)
	h.w.d.Core = appcore.New(appcore.Deps{
		Sources: func() ([]source.Source, string, error) { return nil, "", errors.New("no home") },
		Now:     time.Now,
	})
	h.w.Check(context.Background())
	if st := h.w.Status(); st.Err != "no home" || st.Checking {
		t.Fatalf("status = %+v", st)
	}
}

func TestNotifyFailureIsRetried(t *testing.T) {
	h := newHarness(t, npm(15*gb))
	calls := 0
	h.w.d.Notify = func(string, string) error { calls++; return errors.New("not allowed") }
	h.w.Check(context.Background())
	h.w.Check(context.Background())
	if calls != 2 || !h.w.Settings().LastNotified.IsZero() {
		t.Fatalf("calls = %d, settings = %+v", calls, h.w.Settings())
	}
	h.w.d.Notify = nil
	h.w.Check(context.Background()) // no notifier: nothing happens
}

func TestRun(t *testing.T) {
	src := npm(gb)
	h := newHarness(t, src)
	h.w.d.StartDelay = time.Hour // only CheckNow should trigger checks
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.w.Run(ctx); close(done) }()
	h.w.CheckNow()
	h.w.CheckNow() // coalesced
	deadline := time.After(5 * time.Second)
	for h.w.Status().Checked.IsZero() {
		select {
		case <-deadline:
			t.Fatal("no check ran")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestDescribeAndDetail(t *testing.T) {
	set := Defaults()
	cases := []struct {
		st   Status
		want string
	}{
		{Status{Checking: true}, "Checking…"},
		{Status{Err: "boom\nmore"}, "Couldn't check: boom"},
		{Status{Err: strings.Repeat("x", 80)}, "Couldn't check: " + strings.Repeat("x", 57) + "…"},
		{Status{}, "Not checked yet"},
		{Status{Checked: t0}, "Nothing old to free"},
		{Status{Checked: t0, PrunableBytes: 3 * gb}, "3.0 GB can be freed"},
	}
	for _, c := range cases {
		if got := Describe(c.st); got != c.want {
			t.Errorf("Describe(%+v) = %q, want %q", c.st, got, c.want)
		}
	}
	if got := Detail(Status{}, set, t0); got != "Checks every 6 hours" {
		t.Error(got)
	}
	st := Status{Checked: t0, Bytes: 40 * gb}
	if got := Detail(st, set, t0.Add(time.Hour)); got != "Unused for 30 days · of 40.0 GB · checked 09:00" {
		t.Error(got)
	}
	if got := Detail(st, set, t0.Add(48*time.Hour)); !strings.Contains(got, "Nov 12, 09:00") {
		t.Error(got)
	}
}
