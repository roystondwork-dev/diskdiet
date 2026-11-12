package source

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // cacache names index lines with SHA-1; not used for security
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/fsx"
)

// npmSource is npm's cache: _cacache (package tarballs and metadata), _npx
// (packages run with npx) and _logs.
type npmSource struct{ env Env }

// NewNPM finds npm's cache.
func NewNPM(env Env) Source { return &npmSource{env: env} }

func (s *npmSource) ID() string   { return "npm" }
func (s *npmSource) Name() string { return "npm cache" }

// dir is npm's cache folder: $npm_config_cache, `npm config get cache`, or the
// default (~/.npm, %LOCALAPPDATA%\npm-cache).
func (s *npmSource) dir(ctx context.Context) string {
	for _, k := range []string{"npm_config_cache", "NPM_CONFIG_CACHE"} {
		if v := s.env.Getenv(k); v != "" {
			return v
		}
	}
	if _, err := s.env.Run.LookPath("npm"); err == nil {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if out, err := s.env.Run.Run(ctx, "npm", "config", "get", "cache"); err == nil {
			if p := strings.TrimSpace(string(out)); p != "" && filepath.IsAbs(p) {
				return p
			}
		}
	}
	if s.env.GOOS == "windows" {
		return filepath.Join(s.env.localAppData(), "npm-cache")
	}
	return filepath.Join(s.env.Home, ".npm")
}

type npmPlan struct {
	cacache *cacachePlan
	others  []entryPlan
}

func (s *npmSource) Scan(ctx context.Context, o Options) *Report {
	r := &Report{ID: s.ID(), Name: s.Name()}
	dir := s.dir(ctx)
	if !fsx.IsDir(dir) {
		return r
	}
	r.Found = true
	r.Locations = []string{dir}
	total, _ := fsx.DirSize(ctx, dir, fsx.Options{})
	r.Bytes = total.Bytes

	plan := &npmPlan{}
	if cp, err := scanCacache(ctx, filepath.Join(dir, "_cacache"), o); err != nil {
		r.note("could not read _cacache: " + err.Error())
	} else if cp != nil {
		plan.cacache = cp
		r.Items += cp.items
		r.PrunableBytes += cp.bytes
		r.PrunableItems += cp.count()
	}
	// _npx and _logs are measured like any folder of entries; the scratch
	// report keeps their sizes out of r.Bytes, which already counts them.
	scratch := &Report{}
	plan.others = scanLocations(ctx, scratch, []location{
		{root: filepath.Join(dir, "_npx")},
		{root: filepath.Join(dir, "_logs"), files: true},
	}, o)
	r.Items += scratch.Items
	r.PrunableBytes += scratch.PrunableBytes
	r.PrunableItems += scratch.PrunableItems
	r.plan = plan
	return r
}

func (s *npmSource) Clean(ctx context.Context, r *Report, _ Options) *Result {
	res := &Result{ID: s.ID(), Name: s.Name()}
	plan, _ := r.plan.(*npmPlan)
	if plan == nil {
		return res
	}
	if plan.cacache != nil {
		plan.cacache.apply(ctx, res)
	}
	removeEntries(ctx, plan.others, res)
	return res
}

// cacache layout (https://github.com/npm/cacache):
//
//	index-v5/<2>/<2>/<rest>        buckets: lines of "<sha1 of json>\t<json>"
//	content-v2/<algo>/<2>/<2>/<rest>  content addressed by integrity hash
//	tmp/                            files being written
//
// An index line's JSON holds the key, the integrity of its content and the
// time it was written (ms since epoch). Lines are appended; the last line for
// a key wins and a null integrity deletes the key.

type cacacheEntry struct {
	Key       string  `json:"key"`
	Integrity *string `json:"integrity"`
	Time      int64   `json:"time"`
}

type bucketPlan struct {
	path string
	// keep holds the raw lines to keep; empty means delete the bucket.
	keep  []string
	bytes int64
}

type cacachePlan struct {
	root    string
	buckets []bucketPlan
	content []fsx.Entry
	tmp     []fsx.Entry
	items   int
	bytes   int64
}

func (p *cacachePlan) count() int { return len(p.content) + len(p.tmp) }

// hashAlgos are cacache's algorithms, strongest first; content is stored under
// the strongest one in an entry's integrity.
var hashAlgos = []string{"sha512", "sha384", "sha256", "sha1", "md5"}

// contentPath maps an integrity string to its file under content-v2.
func contentPath(root, integrity string) string {
	byAlgo := map[string]string{}
	for _, tok := range strings.Fields(integrity) {
		algo, digest, ok := strings.Cut(tok, "-")
		if !ok {
			continue
		}
		if i := strings.IndexByte(digest, '?'); i >= 0 {
			digest = digest[:i]
		}
		if _, dup := byAlgo[algo]; !dup {
			byAlgo[algo] = digest
		}
	}
	for _, algo := range hashAlgos {
		d, ok := byAlgo[algo]
		if !ok {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(d)
		if err != nil || len(raw) < 3 {
			return ""
		}
		h := hex.EncodeToString(raw)
		return filepath.Join(root, "content-v2", algo, h[:2], h[2:4], h[4:])
	}
	return ""
}

// validLine checks an index line's SHA-1 prefix and returns its JSON.
func validLine(line string) (body string, ok bool) {
	hash, body, ok := strings.Cut(line, "\t")
	if !ok {
		return "", false
	}
	sum := sha1.Sum([]byte(body)) //nolint:gosec // matches cacache's line format
	return body, hex.EncodeToString(sum[:]) == hash
}

// scanCacache plans which index entries and content files are older than the
// cutoff. Content is removed only when every entry pointing at it is old;
// content no entry points at is removed when the file itself is old.
func scanCacache(ctx context.Context, root string, o Options) (*cacachePlan, error) {
	if !fsx.IsDir(root) {
		return nil, nil
	}
	plan := &cacachePlan{root: root}
	// newest time an entry referenced each content file
	refs := map[string]time.Time{}
	indexDir := filepath.Join(root, "index-v5")
	err := filepath.WalkDir(indexDir, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if p == indexDir && os.IsNotExist(err) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, readErr := os.ReadFile(p) //nolint:gosec // G304: reading npm's own index files
		if readErr != nil {
			return nil //nolint:nilerr // an unreadable bucket is left alone
		}
		info, _ := d.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		bp := planBucket(p, data, o, refs, root)
		if bp != nil {
			bp.bytes = size
			plan.buckets = append(plan.buckets, *bp)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	contentDir := filepath.Join(root, "content-v2")
	files, err := fsx.Files(ctx, contentDir, fsx.Options{})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, f := range files {
		plan.items++
		t, referenced := refs[f.Path]
		if !referenced {
			t = f.Modified
		}
		if o.Old(t) {
			plan.content = append(plan.content, f)
			plan.bytes += f.Usage.Exclusive
		}
	}
	tmp, _ := fsx.Children(ctx, filepath.Join(root, "tmp"), fsx.Options{}, nil)
	for _, e := range tmp {
		if o.Old(e.Modified) {
			plan.tmp = append(plan.tmp, e)
			plan.bytes += e.Usage.Exclusive
		}
	}
	for _, b := range plan.buckets {
		if len(b.keep) == 0 {
			plan.bytes += b.bytes
		}
	}
	return plan, ctx.Err()
}

// planBucket decides which lines of one index bucket to keep. It records, for
// every live key, when its content was last referenced. It returns nil when
// the bucket stays as it is.
func planBucket(path string, data []byte, o Options, refs map[string]time.Time, root string) *bucketPlan {
	type keyState struct {
		last  cacacheEntry
		lines []string
	}
	keys := map[string]*keyState{}
	var order []string
	var invalid bool
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		body, ok := validLine(line)
		if !ok {
			invalid = true
			continue
		}
		var e cacacheEntry
		if json.Unmarshal([]byte(body), &e) != nil {
			invalid = true
			continue
		}
		ks := keys[e.Key]
		if ks == nil {
			ks = &keyState{}
			keys[e.Key] = ks
			order = append(order, e.Key)
		}
		ks.last = e
		ks.lines = append(ks.lines, line)
	}
	var keep []string
	changed := invalid
	for _, k := range order {
		ks := keys[k]
		live := ks.last.Integrity != nil && *ks.last.Integrity != ""
		t := time.UnixMilli(ks.last.Time)
		if live {
			// Content is judged by the newest entry pointing at it, so
			// content shared with a newer key is kept.
			if cp := contentPath(root, *ks.last.Integrity); cp != "" && t.After(refs[cp]) {
				refs[cp] = t
			}
		}
		if live && !o.Old(t) {
			keep = append(keep, ks.lines...)
			continue
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return &bucketPlan{path: path, keep: keep}
}

func (p *cacachePlan) apply(ctx context.Context, res *Result) {
	// Index first, so npm never finds an entry whose content is gone.
	for _, b := range p.buckets {
		if ctx.Err() != nil {
			res.fail(ctx.Err())
			return
		}
		if len(b.keep) == 0 {
			if err := fsx.Remove(p.root, b.path); err != nil {
				res.fail(err)
				continue
			}
			res.Freed += b.bytes
			continue
		}
		res.fail(rewriteBucket(b.path, b.keep))
	}
	removeEntries(ctx, []entryPlan{{root: p.root, entries: p.content}, {root: p.root, entries: p.tmp}}, res)
	pruneEmptyDirs(filepath.Join(p.root, "index-v5"))
	pruneEmptyDirs(filepath.Join(p.root, "content-v2"))
}

// rewriteBucket replaces a bucket with the given lines, atomically.
func rewriteBucket(path string, lines []string) error {
	var buf bytes.Buffer
	for _, l := range lines {
		buf.WriteByte('\n')
		buf.WriteString(l)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".diskdiet-*")
	if err != nil {
		return fmt.Errorf("rewriting %s: %w", path, err)
	}
	_, err = tmp.Write(buf.Bytes())
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("rewriting %s: %w", path, err)
	}
	return nil
}

// pruneEmptyDirs removes empty folders below root, deepest first.
func pruneEmptyDirs(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && p != root {
			dirs = append(dirs, p)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		_ = os.Remove(d) // fails, harmlessly, when not empty
	}
}
