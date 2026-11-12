// Command gendocs writes diskdiet's CLI reference as Markdown for the docs
// site. Run it with `make docs-cli`; CI checks the result is up to date.
package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/roystondwork-dev/diskdiet/internal/cli"
)

func main() {
	out := flag.String("out", "docs/commands", "directory for the Markdown files")
	flag.Parse()
	if err := genMarkdown(cli.NewRootCmd(cli.Deps{}), *out); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

func genMarkdown(root *cobra.Command, dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	root.DisableAutoGenTag = true // no dates, so the files only change when commands do
	for _, c := range root.Commands() {
		c.DisableAutoGenTag = true
		for _, sub := range c.Commands() {
			sub.DisableAutoGenTag = true
		}
	}
	prepend := func(string) string { return "" }
	link := func(name string) string { return strings.TrimSuffix(path.Base(name), ".md") + ".md" }
	if err := doc.GenMarkdownTreeCustom(root, dir, prepend, link); err != nil {
		return fmt.Errorf("generate markdown: %w", err)
	}
	return nil
}
