package source

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

// Mobile and macOS tooling: the Android SDK, Homebrew's downloads, CocoaPods,
// Swift Package Manager and Xcode's simulators.

// androidSDK is the Android SDK folder: $ANDROID_HOME, $ANDROID_SDK_ROOT, or
// where Android Studio puts it.
func androidSDK(env Env) string {
	if v := firstPath(env.Getenv("ANDROID_HOME"), env.Getenv("ANDROID_SDK_ROOT")); v != "" {
		return v
	}
	switch env.GOOS {
	case "darwin":
		return filepath.Join(env.Home, "Library", "Android", "sdk")
	case "windows":
		return filepath.Join(env.localAppData(), "Android", "Sdk")
	default:
		return filepath.Join(env.Home, "Android", "Sdk")
	}
}

// avdHome is where emulators (AVDs) are kept.
func avdHome(env Env) string {
	if v := env.Getenv("ANDROID_AVD_HOME"); v != "" {
		return v
	}
	if v := env.Getenv("ANDROID_USER_HOME"); v != "" {
		return filepath.Join(v, "avd")
	}
	return filepath.Join(env.Home, ".android", "avd")
}

// usedSystemImages returns the system-image folders the emulators use, from
// each AVD's config.ini (image.sysdir.1=system-images/android-34/google_apis/arm64-v8a/).
func usedSystemImages(env Env, sdk string) map[string]bool {
	used := map[string]bool{}
	configs, _ := filepath.Glob(filepath.Join(avdHome(env), "*.avd", "config.ini"))
	for _, c := range configs {
		f, err := os.Open(c) //nolint:gosec // G304: reading the user's own AVD configs
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			key, val, ok := strings.Cut(sc.Text(), "=")
			if !ok || !strings.HasPrefix(strings.TrimSpace(key), "image.sysdir.") {
				continue
			}
			rel := filepath.FromSlash(strings.Trim(strings.TrimSpace(val), "/\\"))
			if rel != "" {
				used[filepath.Clean(filepath.Join(sdk, rel))] = true
			}
		}
		_ = f.Close()
	}
	return used
}

// NewAndroid finds the Android SDK's system images, NDKs and build tools.
// System images an emulator uses always stay, as do the newest NDK and the
// newest build tools; Android Studio and Gradle download others again.
func NewAndroid(env Env) Source {
	return &dirSource{id: "android", name: "Android SDK", locate: func(context.Context) []location {
		sdk := androidSDK(env)
		images := filepath.Join(sdk, "system-images")
		used := usedSystemImages(env, sdk)
		return []location{
			{root: images, show: sdk,
				// system-images/<api>/<tag>/<abi>: one image per folder three levels down.
				find: func(path, _ string) bool {
					rel, err := filepath.Rel(images, path)
					return err == nil && len(strings.Split(rel, string(filepath.Separator))) == 3
				},
				keep: func(entries []fsx.Entry) map[string]bool {
					keep := map[string]bool{}
					for _, e := range entries {
						if used[filepath.Clean(e.Path)] {
							keep[e.Path] = true
						}
					}
					return keep
				}},
			{root: filepath.Join(sdk, "ndk"), skip: hidden, keep: keepNewest, show: sdk},
			{root: filepath.Join(sdk, "build-tools"), skip: hidden, keep: keepNewest, show: sdk},
		}
	}}
}

// homebrewSource is Homebrew's download cache. Installed software is never
// touched; brew downloads a bottle again if it needs one.
type homebrewSource struct {
	dirSource
	env Env
}

// NewHomebrew finds Homebrew's download cache.
func NewHomebrew(env Env) Source {
	s := &homebrewSource{env: env}
	s.dirSource = dirSource{id: "homebrew", name: "Homebrew downloads", locate: func(ctx context.Context) []location {
		return []location{{root: s.cacheDir(ctx), files: true}}
	}}
	return s
}

func (s *homebrewSource) cacheDir(ctx context.Context) string {
	if v := s.env.Getenv("HOMEBREW_CACHE"); v != "" {
		return v
	}
	if _, err := s.env.Run.LookPath("brew"); err == nil {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if out, err := s.env.Run.Run(ctx, "brew", "--cache"); err == nil {
			if p := strings.TrimSpace(string(out)); filepath.IsAbs(p) {
				return p
			}
		}
	}
	if s.env.GOOS == "darwin" {
		return filepath.Join(s.env.Home, "Library", "Caches", "Homebrew")
	}
	return filepath.Join(s.env.cacheHome(), "Homebrew")
}

// Clean removes the planned downloads, then the shortcuts Homebrew keeps to
// them, which would otherwise point nowhere.
func (s *homebrewSource) Clean(ctx context.Context, r *Report, o Options) *Result {
	res := s.dirSource.Clean(ctx, r, o)
	plans, _ := r.plan.([]entryPlan)
	for _, p := range plans {
		removeDanglingLinks(p.root)
	}
	return res
}

// removeDanglingLinks deletes symlinks directly in root whose target is gone.
func removeDanglingLinks(root string) {
	des, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, de := range des {
		if de.Type()&fs.ModeSymlink == 0 {
			continue
		}
		p := filepath.Join(root, de.Name())
		if _, err := os.Stat(p); err != nil {
			_ = os.Remove(p) // the link itself, never its (missing) target
		}
	}
}

// NewCocoaPods finds CocoaPods' download cache: one folder per pod version.
// Projects' own Pods folders are never touched.
func NewCocoaPods(env Env) Source {
	return &dirSource{id: "cocoapods", name: "CocoaPods cache", locate: func(context.Context) []location {
		root := filepath.Join(env.cacheHome(), "CocoaPods")
		if env.GOOS == "windows" {
			return nil
		}
		var locs []location
		for _, kind := range []string{"Release", "External"} {
			for _, d := range globDirs(filepath.Join(root, "Pods", kind, "*")) {
				locs = append(locs, location{root: d, skip: hidden, show: root})
			}
		}
		return locs
	}}
}

// NewSwiftPM finds Swift Package Manager's cache of package repositories and
// binary artifacts.
func NewSwiftPM(env Env) Source {
	return &dirSource{id: "swiftpm", name: "Swift Package Manager cache", locate: func(context.Context) []location {
		if env.GOOS == "windows" {
			return nil
		}
		root := filepath.Join(env.cacheHome(), "org.swift.swiftpm")
		return []location{
			{root: filepath.Join(root, "repositories"), skip: hidden, show: root},
			{root: filepath.Join(root, "artifacts"), skip: hidden, show: root},
		}
	}}
}

// simulatorsSource is Xcode's simulators: devices for runtimes that are no
// longer installed, which `xcrun simctl delete unavailable` removes, and the
// simulator caches, which are rebuilt. macOS only.
type simulatorsSource struct{ env Env }

// NewSimulators finds unavailable simulators and old simulator caches.
func NewSimulators(env Env) Source { return &simulatorsSource{env: env} }

func (s *simulatorsSource) ID() string   { return "simulators" }
func (s *simulatorsSource) Name() string { return "Xcode simulators" }

type simDevice struct {
	UDID        string `json:"udid"`
	Name        string `json:"name"`
	DataPath    string `json:"dataPath"`
	IsAvailable bool   `json:"isAvailable"`
}

type simPlan struct {
	devices []string // device folders that `simctl delete unavailable` removes
	caches  []entryPlan
}

func (s *simulatorsSource) coreSimulator() string {
	return filepath.Join(s.env.Home, "Library", "Developer", "CoreSimulator")
}

// unavailable lists the device folders of unavailable simulators.
func (s *simulatorsSource) unavailable(ctx context.Context) ([]string, bool) {
	if _, err := s.env.Run.LookPath("xcrun"); err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	out, err := s.env.Run.Run(ctx, "xcrun", "simctl", "list", "devices", "unavailable", "-j")
	if err != nil {
		return nil, false // no full Xcode: simctl isn't there
	}
	var doc struct {
		Devices map[string][]simDevice `json:"devices"`
	}
	if json.Unmarshal(out, &doc) != nil {
		return nil, false
	}
	devices := filepath.Join(s.coreSimulator(), "Devices")
	var dirs []string
	for _, list := range doc.Devices {
		for _, d := range list {
			if d.IsAvailable || d.DataPath == "" {
				continue
			}
			dir := filepath.Dir(filepath.Clean(d.DataPath)) // …/Devices/<UDID>/data → …/Devices/<UDID>
			if fsx.Within(devices, dir) && dir != devices {
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs, true
}

func (s *simulatorsSource) Scan(ctx context.Context, o Options) *Report {
	r := &Report{ID: s.ID(), Name: s.Name()}
	if s.env.GOOS != "darwin" {
		return r
	}
	plan := &simPlan{}
	devices, ok := s.unavailable(ctx)
	if ok && len(devices) > 0 {
		r.Found = true
		for _, d := range devices {
			u, _ := fsx.DirSize(ctx, d, fsx.Options{})
			r.Bytes += u.Bytes
			r.Items++
			r.PrunableBytes += u.Exclusive
			r.PrunableItems++
		}
		plan.devices = devices
		if o.Age > 0 {
			r.note("simulators for runtimes that are no longer installed are removed whatever their age")
		}
	}
	plan.caches = scanLocations(ctx, r, []location{{root: filepath.Join(s.coreSimulator(), "Caches"), skip: hidden}}, o)
	if r.Found {
		r.Locations = []string{s.coreSimulator()} // devices and caches both live here
	}
	r.plan = plan
	return r
}

func (s *simulatorsSource) Clean(ctx context.Context, r *Report, _ Options) *Result {
	res := &Result{ID: s.ID(), Name: s.Name()}
	plan, _ := r.plan.(*simPlan)
	if plan == nil {
		return res
	}
	if len(plan.devices) > 0 {
		var before int64
		for _, d := range plan.devices {
			u, _ := fsx.DirSize(ctx, d, fsx.Options{})
			before += u.Bytes
		}
		ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
		_, err := s.env.Run.Run(ctx2, "xcrun", "simctl", "delete", "unavailable")
		cancel()
		res.fail(err)
		var after int64
		for _, d := range plan.devices {
			if u, err := fsx.DirSize(ctx, d, fsx.Options{}); err == nil {
				after += u.Bytes
			} else {
				res.Removed++
			}
		}
		if freed := before - after; freed > 0 {
			res.Freed += freed
		}
		if err == nil {
			res.Notes = append(res.Notes, "ran xcrun simctl delete unavailable")
		}
	}
	removeEntries(ctx, plan.caches, res)
	return res
}
