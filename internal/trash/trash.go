// Package trash moves files and folders to the system Trash (macOS), the
// Recycle Bin (Windows) or the freedesktop.org Trash (Linux), so they can be
// put back. Nothing is freed until the Trash is emptied.
package trash

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// ErrUnsupported is returned where there is no Trash to move to.
var ErrUnsupported = errors.New("moving to the Trash isn't supported here")

// runFunc runs a program and returns its combined output. Replaced in tests.
var runFunc = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec // G204: fixed system tools
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// Move puts path in the Trash.
func Move(ctx context.Context, path string) error {
	return moveOn(ctx, runtime.GOOS, path, os.UserHomeDir, os.Getenv)
}

// moveOn is Move for a given OS, so every branch is testable anywhere.
func moveOn(ctx context.Context, goos, path string, homeDir func() (string, error), getenv func(string) string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("moving %s to the Trash: %w", path, err)
	}
	switch goos {
	case "darwin":
		return macOS(ctx, abs)
	case "windows":
		return windows(ctx, abs)
	case "linux", "freebsd", "openbsd", "netbsd":
		dataHome := getenv("XDG_DATA_HOME")
		if dataHome == "" {
			home, err := homeDir()
			if err != nil {
				return fmt.Errorf("moving %s to the Trash: %w", path, err)
			}
			dataHome = filepath.Join(home, ".local", "share")
		}
		return freedesktop(filepath.Join(dataHome, "Trash"), abs, time.Now())
	default:
		return ErrUnsupported
	}
}

// macOSScript asks Foundation's NSFileManager to trash the item, as Finder
// does, so Put Back works. No Finder automation permission is needed.
const macOSScript = `ObjC.import("Foundation");
function run(argv) {
  var ok = $.NSFileManager.defaultManager.trashItemAtURLResultingItemURLError($.NSURL.fileURLWithPath(argv[0]), null, null);
  return ok ? "ok" : "failed";
}`

func macOS(ctx context.Context, path string) error {
	out, err := runFunc(ctx, "osascript", "-l", "JavaScript", "-e", macOSScript, path)
	if err != nil {
		return fmt.Errorf("moving %s to the Trash: %w", path, err)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		return fmt.Errorf("moving %s to the Trash: macOS refused", path)
	}
	return nil
}

// windowsScript sends a file or folder to the Recycle Bin through the .NET
// API Explorer uses. %s is the path as a single-quoted PowerShell literal.
const windowsScript = `$p = %s
Add-Type -AssemblyName Microsoft.VisualBasic
if (Test-Path -LiteralPath $p -PathType Container) {
  [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteDirectory($p, 'OnlyErrorDialogs', 'SendToRecycleBin')
} else {
  [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($p, 'OnlyErrorDialogs', 'SendToRecycleBin')
}`

// psQuote makes s a single-quoted PowerShell string literal.
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// encodeCommand encodes a script for powershell -EncodedCommand (UTF-16LE,
// base64), so no quoting reaches the command line.
func encodeCommand(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func windows(ctx context.Context, path string) error {
	script := fmt.Sprintf(windowsScript, psQuote(path))
	if _, err := runFunc(ctx, "powershell", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodeCommand(script)); err != nil {
		return fmt.Errorf("moving %s to the Recycle Bin: %w", path, err)
	}
	return nil
}

// freedesktop moves path into trashDir following the freedesktop.org Trash
// specification: the item goes to files/, and info/<name>.trashinfo records
// where it came from, so file managers can restore it. The Trash must be on
// the same disk; diskdiet never copies-and-deletes.
func freedesktop(trashDir, path string, now time.Time) error {
	files, info := filepath.Join(trashDir, "files"), filepath.Join(trashDir, "info")
	for _, d := range []string{files, info} {
		if err := os.MkdirAll(d, 0o700); err != nil { //nolint:gosec // G703: the Trash folder and a path the caller checked is inside its project
			return fmt.Errorf("moving %s to the Trash: %w", path, err)
		}
	}
	base := filepath.Base(path)
	for i := 0; ; i++ {
		name := base
		if i > 0 {
			name = base + "." + strconv.Itoa(i)
		}
		infoPath := filepath.Join(info, name+".trashinfo")
		// Claim the name by creating the info file exclusively.
		f, err := os.OpenFile(infoPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // G304: inside the Trash
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("moving %s to the Trash: %w", path, err)
		}
		u := url.URL{Path: path}
		_, err = fmt.Fprintf(f, "[Trash Info]\nPath=%s\nDeletionDate=%s\n", u.EscapedPath(), now.Format("2006-01-02T15:04:05"))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(path, filepath.Join(files, name)) //nolint:gosec // G703: the Trash folder and a path the caller checked is inside its project
		}
		if err != nil {
			_ = os.Remove(infoPath) //nolint:gosec // G703: the Trash folder and a path the caller checked is inside its project
			return fmt.Errorf("moving %s to the Trash (it must be on the same disk as %s): %w", path, trashDir, err)
		}
		return nil
	}
}
