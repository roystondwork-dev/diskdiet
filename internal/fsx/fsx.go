// Package fsx measures disk usage and removes cache entries safely.
package fsx

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Usage is the disk space used by a file tree.
type Usage struct {
	// Bytes is the space the tree takes on disk. Files hard-linked more than
	// once inside the tree are counted once.
	Bytes int64 `json:"bytes"`
	// Exclusive is the part of Bytes that deleting the tree would free: files
	// with no hard links elsewhere (for example from a pnpm store into a
	// project's node_modules). Equal to Bytes when links are not counted.
	Exclusive int64 `json:"exclusiveBytes"`
	// Files is the number of regular files.
	Files int `json:"files"`
	// ExclusiveFiles is the number of regular files counted in Exclusive.
	// Folders take space too (4 kB each on Linux), so Exclusive alone can't
	// tell an empty tree from one with files to free.
	ExclusiveFiles int `json:"exclusiveFiles"`
}

// Add accumulates u2 into u.
func (u *Usage) Add(u2 Usage) {
	u.Bytes += u2.Bytes
	u.Exclusive += u2.Exclusive
	u.Files += u2.Files
	u.ExclusiveFiles += u2.ExclusiveFiles
}

// Options control measurement.
type Options struct {
	// CountLinks makes Exclusive leave out hard-linked files. Where the OS
	// cannot report link counts cheaply (Windows), every file is exclusive.
	CountLinks bool
}

// fileID identifies a file for hard-link de-duplication.
type fileID struct{ dev, ino uint64 }

// DirSize measures the tree rooted at path. Symlinks are not followed.
// Unreadable entries are skipped; the first such error is returned with the
// partial result.
func DirSize(ctx context.Context, path string, opts Options) (Usage, error) {
	var u Usage
	var firstErr error
	seen := map[fileID]bool{}
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if d != nil && d.IsDir() && p != path {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		st := stat(p, info, opts.CountLinks)
		if st.links > 1 && st.id != (fileID{}) {
			if seen[st.id] {
				return nil
			}
			seen[st.id] = true
		}
		u.Bytes += st.bytes
		if info.Mode().IsRegular() {
			u.Files++
		}
		if !opts.CountLinks || st.links <= 1 || info.IsDir() {
			u.Exclusive += st.bytes
			if info.Mode().IsRegular() {
				u.ExclusiveFiles++
			}
		}
		return nil
	})
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return u, err
	}
	if errors.Is(err, fs.ErrNotExist) {
		return u, err
	}
	return u, firstErr
}

// Entry is one removable item in a cache: a package folder, a file or a project's
// node_modules.
type Entry struct {
	Path     string    `json:"path"`
	Usage    Usage     `json:"usage"`
	Modified time.Time `json:"modified"`
}

// Children measures each direct child of dir as one Entry. The entry's age is
// the child's own modification time.
func Children(ctx context.Context, dir string, opts Options, skip func(name string) bool) ([]Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}
	var out []Entry
	for _, de := range des {
		if ctx.Err() != nil {
			return out, fmt.Errorf("measuring %s: %w", dir, ctx.Err())
		}
		if skip != nil && skip(de.Name()) {
			continue
		}
		p := filepath.Join(dir, de.Name())
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		u, _ := DirSize(ctx, p, opts)
		out = append(out, Entry{Path: p, Usage: u, Modified: info.ModTime()})
	}
	if err := ctx.Err(); err != nil {
		return out, fmt.Errorf("measuring %s: %w", dir, err)
	}
	return out, nil
}

// Files measures every regular file under dir as one Entry.
func Files(ctx context.Context, dir string, opts Options) ([]Entry, error) {
	var out []Entry
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if p == dir {
				return err
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil //nolint:nilerr // a file that vanished mid-walk is skipped
		}
		st := stat(p, info, opts.CountLinks)
		u := Usage{Bytes: st.bytes, Exclusive: st.bytes, Files: 1}
		if opts.CountLinks && st.links > 1 {
			u.Exclusive = 0
		}
		out = append(out, Entry{Path: p, Usage: u, Modified: info.ModTime()})
		return nil
	})
	if err != nil {
		return out, fmt.Errorf("listing %s: %w", dir, err)
	}
	return out, nil
}

// ErrOutsideRoot is returned when asked to remove a path outside its cache.
var ErrOutsideRoot = errors.New("path is outside the cache folder")

// Within reports whether path is root or inside it.
func Within(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// Remove deletes path, which must lie strictly inside root. It never follows
// symlinks. Read-only files and folders (as left by some package managers) are
// made writable and the removal retried.
func Remove(root, path string) error {
	if err := checkRoot(root); err != nil {
		return err
	}
	if !Within(root, path) || filepath.Clean(root) == filepath.Clean(path) {
		return fmt.Errorf("%w: %s", ErrOutsideRoot, path)
	}
	if err := os.RemoveAll(path); err == nil {
		return nil
	}
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink != 0 {
			return nil //nolint:nilerr // best effort: unreadable entries fail the retry below
		}
		if d.IsDir() {
			_ = os.Chmod(p, 0o700) //nolint:gosec // G302/G122: owner-only, on our own cache files, to delete them
		} else {
			_ = os.Chmod(p, 0o600) //nolint:gosec // G122: owner-only, on our own cache files, to delete them
		}
		return nil
	})
	_ = os.Chmod(filepath.Dir(path), dirModeWritable(filepath.Dir(path)))
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

func dirModeWritable(p string) fs.FileMode {
	info, err := os.Lstat(p)
	if err != nil {
		return 0o700
	}
	return info.Mode().Perm() | 0o700
}

// checkRoot refuses roots that could not be a cache folder: relative paths, a
// filesystem root or the home folder itself.
func checkRoot(root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("refusing to remove from %q: not an absolute path", root)
	}
	clean := filepath.Clean(root)
	if filepath.Dir(clean) == clean {
		return fmt.Errorf("refusing to remove from %q: filesystem root", root)
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(home) == clean {
		return fmt.Errorf("refusing to remove from %q: home folder", root)
	}
	return nil
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// IsDir reports whether path is a directory (following symlinks).
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Find walks root and measures each folder that accept chooses as one Entry,
// without descending into it. It never follows symlinks. The entry's age is
// the folder's own modification time.
func Find(ctx context.Context, root string, opts Options, accept func(path, name string) bool) ([]Entry, error) {
	var out []Entry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if p == root {
				return err
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if p == root || !d.IsDir() {
			return nil
		}
		if !accept(p, d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fs.SkipDir
		}
		u, _ := DirSize(ctx, p, opts)
		out = append(out, Entry{Path: p, Usage: u, Modified: info.ModTime()})
		return fs.SkipDir
	})
	if err != nil {
		return out, fmt.Errorf("searching %s: %w", root, err)
	}
	return out, nil
}
