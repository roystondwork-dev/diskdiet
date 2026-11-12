package runner

import (
	"context"
	"strings"
	"testing"
)

// These tests run the go command, which every build machine has.
func TestSystem(t *testing.T) {
	var r Runner = System{}
	if _, err := r.LookPath("go"); err != nil {
		t.Skip("go not on PATH:", err)
	}
	if _, err := r.LookPath("diskdiet-no-such-command"); err == nil {
		t.Error("LookPath of missing command: want error")
	}
	out, err := r.Run(context.Background(), "go", "version")
	if err != nil || !strings.HasPrefix(string(out), "go version") {
		t.Fatalf("go version = %q, %v", out, err)
	}
	_, err = r.Run(context.Background(), "go", "no-such-subcommand")
	if err == nil || !strings.Contains(err.Error(), "go no-such-subcommand") || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("failing command error = %v", err)
	}
	_, err = r.Run(context.Background(), "diskdiet-no-such-command")
	if err == nil {
		t.Error("missing command: want error")
	}
}

func TestFirstLine(t *testing.T) {
	if firstLine("a\nb") != "a" || firstLine("a") != "a" {
		t.Error("firstLine")
	}
}
