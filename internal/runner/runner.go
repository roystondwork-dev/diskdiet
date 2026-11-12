// Package runner runs external commands (npm, pnpm, docker) behind an interface
// so that sources can be tested without those tools installed.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs commands.
type Runner interface {
	// LookPath reports whether a command is installed and where.
	LookPath(name string) (string, error)
	// Run runs a command and returns its standard output. On failure the error
	// includes the command's standard error.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// System runs real commands.
type System struct{}

// LookPath implements Runner.
func (System) LookPath(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("finding %s: %w", name, err)
	}
	return p, nil
}

// Run implements Runner.
func (System) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: runs fixed package-manager commands
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return stdout.Bytes(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, firstLine(msg))
	}
	return stdout.Bytes(), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
