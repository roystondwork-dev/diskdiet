package source

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

// The caches of Python, Rust, Go, Java, browser-testing and Apple tools. Each
// removes only what its tool downloads or builds again on demand.

// firstPath returns the first non-empty value, for "env var, else default".
func firstPath(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// glob lists the folders matching pattern.
func globDirs(pattern string) []string {
	matches, _ := filepath.Glob(pattern)
	var out []string
	for _, m := range matches {
		if fsx.IsDir(m) {
			out = append(out, m)
		}
	}
	return out
}

// NewPip finds pip's cache: downloaded packages and built wheels.
func NewPip(env Env) Source {
	return &dirSource{id: "pip", name: "pip cache", locate: func(context.Context) []location {
		def := filepath.Join(env.cacheHome(), "pip")
		if env.GOOS == "windows" {
			def = filepath.Join(env.localAppData(), "pip", "Cache")
		}
		return []location{{root: firstPath(env.Getenv("PIP_CACHE_DIR"), def), files: true}}
	}}
}

// NewCargo finds Cargo's registry and git caches. Unpacked sources are
// unpacked again from the downloaded crates, and crates download again.
func NewCargo(env Env) Source {
	return &dirSource{id: "cargo", name: "Cargo cache", locate: func(context.Context) []location {
		home := firstPath(env.Getenv("CARGO_HOME"), filepath.Join(env.Home, ".cargo"))
		locs := make([]location, 0, 4)
		for _, d := range globDirs(filepath.Join(home, "registry", "src", "*")) {
			locs = append(locs, location{root: d, show: home})
		}
		locs = append(locs, location{root: filepath.Join(home, "registry", "cache"), files: true, show: home})
		for _, d := range globDirs(filepath.Join(home, "git", "checkouts", "*")) {
			locs = append(locs, location{root: d, show: home})
		}
		locs = append(locs, location{root: filepath.Join(home, "git", "db"), show: home})
		return locs
	}}
}

// NewGo finds Go's build cache and module cache. The go command marks build
// cache entries as used when it uses them, so their age is their last use.
func NewGo(env Env) Source {
	return &dirSource{id: "go", name: "Go caches", locate: func(ctx context.Context) []location {
		gocache, modcache := env.Getenv("GOCACHE"), env.Getenv("GOMODCACHE")
		if gocache == "" || modcache == "" {
			if _, err := env.Run.LookPath("go"); err == nil {
				ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()
				if out, err := env.Run.Run(ctx, "go", "env", "GOCACHE", "GOMODCACHE"); err == nil {
					lines := strings.Split(strings.TrimSpace(string(out)), "\n")
					if len(lines) == 2 {
						gocache = firstPath(gocache, strings.TrimSpace(lines[0]))
						modcache = firstPath(modcache, strings.TrimSpace(lines[1]))
					}
				}
			}
		}
		if gocache == "" {
			gocache = filepath.Join(env.cacheHome(), "go-build")
			if env.GOOS == "windows" {
				gocache = filepath.Join(env.localAppData(), "go-build")
			}
		}
		if modcache == "" {
			gopath := env.Getenv("GOPATH")
			if i := strings.IndexRune(gopath, os.PathListSeparator); i >= 0 {
				gopath = gopath[:i]
			}
			modcache = filepath.Join(firstPath(gopath, filepath.Join(env.Home, "go")), "pkg", "mod")
		}
		if gocache == "off" {
			gocache = ""
		}
		var locs []location
		if gocache != "" && filepath.IsAbs(gocache) {
			locs = append(locs, location{root: gocache, files: true})
		}
		if filepath.IsAbs(modcache) {
			download := filepath.Join(modcache, "cache")
			locs = append(locs,
				// Each extracted module version: example.com/mod@v1.2.3.
				location{root: modcache, find: func(path, name string) bool {
					return strings.Contains(name, "@") && !fsx.Within(download, path)
				}},
				location{root: filepath.Join(download, "download"), files: true, show: modcache},
			)
		}
		return locs
	}}
}

var gradleVersionDir = regexp.MustCompile(`^\d+\.\d+`)

// NewGradle finds downloaded Gradle versions and per-version caches and logs.
// Gradle manages its shared dependency cache itself.
func NewGradle(env Env) Source {
	return &dirSource{id: "gradle", name: "Gradle caches", locate: func(context.Context) []location {
		home := firstPath(env.Getenv("GRADLE_USER_HOME"), filepath.Join(env.Home, ".gradle"))
		versioned := func(name string) bool { return !gradleVersionDir.MatchString(name) }
		return []location{
			{root: filepath.Join(home, "wrapper", "dists"), show: home},
			{root: filepath.Join(home, "caches"), skip: versioned, show: home},
			{root: filepath.Join(home, "daemon"), skip: versioned, show: home},
		}
	}}
}

// NewMaven finds artifact versions in Maven's local repository.
func NewMaven(env Env) Source {
	return &dirSource{id: "maven", name: "Maven repository", locate: func(context.Context) []location {
		repo := filepath.Join(env.Home, ".m2", "repository")
		return []location{{root: repo, find: func(path, _ string) bool {
			// A version folder holds the artifact's .pom.
			des, err := os.ReadDir(path)
			if err != nil {
				return false
			}
			for _, de := range des {
				if !de.IsDir() && strings.HasSuffix(de.Name(), ".pom") {
					return true
				}
			}
			return false
		}}}
	}}
}

// keepNewest keeps the newest version of each kind of entry: chromium-1140
// and chromium-1148 are one kind, so 1148 stays; 13.6.0 and 12.17.4 are
// versions of one thing, so 13.6.0 stays.
func keepNewest(entries []fsx.Entry) map[string]bool {
	newest := map[string]fsx.Entry{}
	for _, e := range entries {
		kind, _ := splitVersion(filepath.Base(e.Path))
		cur, ok := newest[kind]
		if !ok || compareVersions(filepath.Base(e.Path), filepath.Base(cur.Path)) > 0 {
			newest[kind] = e
		}
	}
	keep := map[string]bool{}
	for _, e := range newest {
		keep[e.Path] = true
	}
	return keep
}

// splitVersion splits "chromium-1140" into ("chromium", "1140") and "13.6.0"
// into ("", "13.6.0"). Names without a version are their own kind.
func splitVersion(name string) (kind, version string) {
	isVersion := func(s string) bool {
		if s == "" {
			return false
		}
		for _, r := range s {
			if (r < '0' || r > '9') && r != '.' {
				return false
			}
		}
		return true
	}
	if isVersion(name) {
		return "", name
	}
	if i := strings.LastIndexByte(name, '-'); i > 0 && isVersion(name[i+1:]) {
		return name[:i], name[i+1:]
	}
	return name, ""
}

// compareVersions compares the versions of two names of one kind.
func compareVersions(a, b string) int {
	_, va := splitVersion(a)
	_, vb := splitVersion(b)
	pa, pb := strings.Split(va, "."), strings.Split(vb, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

// NewPlaywright finds Playwright's browsers. The newest revision of each
// browser always stays, so tests keep running.
func NewPlaywright(env Env) Source {
	return &dirSource{id: "playwright", name: "Playwright browsers", locate: func(context.Context) []location {
		def := filepath.Join(env.cacheHome(), "ms-playwright")
		if env.GOOS == "windows" {
			def = filepath.Join(env.localAppData(), "ms-playwright")
		}
		root := firstPath(env.Getenv("PLAYWRIGHT_BROWSERS_PATH"), def)
		if root == "0" { // browsers live inside node_modules
			return nil
		}
		return []location{{root: root, skip: hidden, keep: keepNewest}}
	}}
}

// NewCypress finds Cypress's downloaded app versions. The newest stays.
func NewCypress(env Env) Source {
	return &dirSource{id: "cypress", name: "Cypress binaries", locate: func(context.Context) []location {
		def := filepath.Join(env.cacheHome(), "Cypress")
		if env.GOOS == "windows" {
			def = filepath.Join(env.localAppData(), "Cypress", "Cache")
		}
		return []location{{root: firstPath(env.Getenv("CYPRESS_CACHE_FOLDER"), def), skip: hidden, keep: keepNewest}}
	}}
}

// NewXcode finds Xcode's build products (DerivedData) and the device support
// files Xcode copies from each iOS, watchOS or tvOS version it meets. Archives
// are never touched. macOS only.
func NewXcode(env Env) Source {
	return &dirSource{id: "xcode", name: "Xcode caches", locate: func(context.Context) []location {
		if env.GOOS != "darwin" {
			return nil
		}
		dev := filepath.Join(env.Home, "Library", "Developer", "Xcode")
		locs := []location{{root: filepath.Join(dev, "DerivedData"), skip: hidden}}
		for _, d := range globDirs(filepath.Join(dev, "*DeviceSupport")) {
			locs = append(locs, location{root: d, skip: hidden})
		}
		return locs
	}}
}

// uvSource is uv's cache. uv's entries point at one another, so diskdiet
// doesn't remove them one by one: with --all it runs `uv cache clean`.
type uvSource struct{ env Env }

// NewUV finds uv's cache.
func NewUV(env Env) Source { return &uvSource{env: env} }

func (s *uvSource) ID() string   { return "uv" }
func (s *uvSource) Name() string { return "uv cache" }

func (s *uvSource) dir(ctx context.Context) string {
	if v := s.env.Getenv("UV_CACHE_DIR"); v != "" {
		return v
	}
	if _, err := s.env.Run.LookPath("uv"); err == nil {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if out, err := s.env.Run.Run(ctx, "uv", "cache", "dir"); err == nil {
			if p := strings.TrimSpace(string(out)); filepath.IsAbs(p) {
				return p
			}
		}
	}
	if s.env.GOOS == "windows" {
		return filepath.Join(s.env.localAppData(), "uv", "cache")
	}
	// uv uses the XDG layout on macOS too.
	return filepath.Join(firstPath(s.env.Getenv("XDG_CACHE_HOME"), filepath.Join(s.env.Home, ".cache")), "uv")
}

type uvPlan struct{ dir string }

func (s *uvSource) Scan(ctx context.Context, o Options) *Report {
	r := &Report{ID: s.ID(), Name: s.Name()}
	dir := s.dir(ctx)
	if !fsx.IsDir(dir) {
		return r
	}
	r.Found = true
	r.Locations = []string{dir}
	u, _ := fsx.DirSize(ctx, dir, fsx.Options{CountLinks: true})
	r.Bytes, r.Items = u.Bytes, u.Files
	if _, err := s.env.Run.LookPath("uv"); err != nil {
		r.note("uv isn't installed, so its cache can't be cleaned safely")
		return r
	}
	if o.Age > 0 {
		r.note("uv's cache is cleared as a whole: choose everything (--all) to clear it")
		return r
	}
	r.PrunableBytes, r.PrunableItems = u.Exclusive, u.Files
	r.plan = &uvPlan{dir: dir}
	return r
}

func (s *uvSource) Clean(ctx context.Context, r *Report, _ Options) *Result {
	res := &Result{ID: s.ID(), Name: s.Name()}
	plan, _ := r.plan.(*uvPlan)
	if plan == nil {
		return res
	}
	before, _ := fsx.DirSize(ctx, plan.dir, fsx.Options{})
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Minute)
	_, err := s.env.Run.Run(ctx2, "uv", "cache", "clean")
	cancel()
	res.fail(err)
	after, _ := fsx.DirSize(ctx, plan.dir, fsx.Options{})
	if freed := before.Bytes - after.Bytes; freed > 0 {
		res.Freed = freed
	}
	if err == nil {
		res.Notes = append(res.Notes, "ran uv cache clean")
	}
	return res
}
