package source

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

// FuzzContentPath: an integrity string read from npm's index (which anyone who
// can write to the cache controls) must never map outside content-v2.
func FuzzContentPath(f *testing.F) {
	for _, seed := range []string{
		"sha512-zjpD9rfMWoy+117YIfYZjgev7xuWTs1M+TCHzmvMLHg+aCeNFOlaldEESF/xiy4umcvGAwyDSjycJAZGzxih2w==",
		"sha1-AAAA sha512-BBBB?opt",
		"../../etc-passwd",
		"sha512-Li4vLi4vLi4v",
		"sha512-",
		"",
	} {
		f.Add(seed)
	}
	root := filepath.Join(f.TempDir(), "_cacache")
	content := filepath.Join(root, "content-v2")
	f.Fuzz(func(t *testing.T, integrity string) {
		p := contentPath(root, integrity)
		if p != "" && (!fsx.Within(content, p) || filepath.Clean(p) == content) {
			t.Fatalf("contentPath(%q) = %q, outside %s", integrity, p, content)
		}
	})
}

// FuzzPlanBucket: any bucket file parses without panicking, keeps only lines
// that were in it, and records only content paths inside the cache.
func FuzzPlanBucket(f *testing.F) {
	in := integrity("x")
	f.Add("\n" + indexLine("k", &in, time.Unix(1, 0)))
	f.Add("\n" + indexLine("k", nil, time.Now()) + "\nnot a line\n")
	f.Add("\nabc\t{\"key\":")
	root := filepath.Join(f.TempDir(), "_cacache")
	now := time.Now()
	f.Fuzz(func(t *testing.T, data string) {
		refs := map[string]time.Time{}
		o := Options{Age: 30 * 24 * time.Hour, Now: now}
		bp := planBucket(filepath.Join(root, "index-v5", "aa", "bb", "x"), []byte(data), o, refs, root)
		if bp != nil {
			lines := map[string]bool{}
			for _, l := range strings.Split(data, "\n") {
				lines[l] = true
			}
			for _, k := range bp.keep {
				if !lines[k] {
					t.Fatalf("kept a line that wasn't in the bucket: %q", k)
				}
			}
		}
		for p := range refs {
			if !fsx.Within(filepath.Join(root, "content-v2"), p) {
				t.Fatalf("content path %q outside the cache", p)
			}
		}
	})
}

// FuzzVersions: version names never panic and compare consistently.
func FuzzVersions(f *testing.F) {
	f.Add("chromium-1140", "chromium-1100")
	f.Add("13.6.0", "9.7.0")
	f.Add("x-", "-1")
	f.Fuzz(func(t *testing.T, a, b string) {
		splitVersion(a)
		if compareVersions(a, b) != -compareVersions(b, a) {
			t.Fatalf("compareVersions(%q, %q) is not antisymmetric", a, b)
		}
	})
}
