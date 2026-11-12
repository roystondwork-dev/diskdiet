// Package cli implements the diskdiet command line.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/roystondwork-dev/diskdiet/internal/buildinfo"
	"github.com/roystondwork-dev/diskdiet/internal/procs"
	"github.com/roystondwork-dev/diskdiet/internal/source"
	"github.com/roystondwork-dev/diskdiet/internal/units"
)

// Exit codes.
const (
	ExitOK      = 0
	ExitError   = 1
	ExitUsage   = 2
	ExitPartial = 3
)

// Deps is what the commands need from the outside world. Tests replace it.
type Deps struct {
	Sources func() ([]source.Source, string, error) // sources and the home folder
	Busy    func(ctx context.Context) (map[string]string, error)
	Now     func() time.Time
	In      io.Reader
	Out     io.Writer
	Err     io.Writer
	// Interactive reports whether In is a terminal a person can answer.
	Interactive bool
	// Progress reports whether Err is a terminal that can show progress.
	Progress bool
}

// SystemDeps uses the real machine and standard streams.
func SystemDeps() Deps {
	return Deps{
		Sources: func() ([]source.Source, string, error) {
			env, err := source.SystemEnv()
			if err != nil {
				return nil, "", err
			}
			return source.All(env), env.Home, nil
		},
		Busy:        procs.Busy,
		Now:         time.Now,
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		Interactive: term.IsTerminal(int(os.Stdin.Fd())),
		Progress:    term.IsTerminal(int(os.Stderr.Fd())),
	}
}

// exitError carries an exit code.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// Execute runs diskdiet with args and returns the exit code.
func Execute(ctx context.Context, d Deps, args []string) int {
	root := newRoot(d)
	root.SetArgs(args)
	root.SetIn(d.In)
	root.SetOut(d.Out)
	root.SetErr(d.Err)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil && ee.err.Error() != "" {
			fmt.Fprintln(d.Err, "diskdiet:", ee.err)
		}
		return ee.code
	}
	fmt.Fprintln(d.Err, "diskdiet:", err)
	if strings.Contains(err.Error(), "unknown command") || strings.Contains(err.Error(), "accepts") {
		return ExitUsage
	}
	return ExitError
}

// common holds the flags shared by scan and clean.
type common struct {
	olderThan string
	all       bool
	only      []string
	skip      []string
	projects  []string
	json      bool
}

func (c *common) register(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&c.olderThan, "older-than", "30d", "count things unchanged for this long as old (e.g. 7d, 2w, 12h)")
	f.BoolVar(&c.all, "all", false, "count everything, whatever its age")
	f.StringSliceVar(&c.only, "only", nil, "only these sources: "+strings.Join(source.IDs(), ", "))
	f.StringSliceVar(&c.skip, "skip", nil, "leave out these sources")
	f.StringArrayVar(&c.projects, "projects", nil, "folder to search for node_modules (repeatable; default: your home folder)")
	f.BoolVar(&c.json, "json", false, "print JSON")
}

// options turns flags into source options.
func (c *common) options(now time.Time) (source.Options, error) {
	o := source.Options{Now: now}
	if !c.all {
		age, err := units.ParseAge(c.olderThan)
		if err != nil {
			return o, &exitError{ExitUsage, err}
		}
		o.Age = age
	}
	for _, p := range c.projects {
		abs, err := absPath(p)
		if err != nil {
			return o, &exitError{ExitUsage, err}
		}
		o.Projects = append(o.Projects, abs)
	}
	return o, nil
}

func (c *common) selected(all []source.Source) ([]source.Source, error) {
	known := source.IDs()
	for _, id := range append(slices.Clone(c.only), c.skip...) {
		if !slices.Contains(known, normalizeID(id)) {
			return nil, &exitError{ExitUsage, fmt.Errorf("unknown source %q; choose from %s", id, strings.Join(known, ", "))}
		}
	}
	var out []source.Source
	for _, s := range all {
		if len(c.only) > 0 && !containsID(c.only, s.ID()) {
			continue
		}
		if containsID(c.skip, s.ID()) {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func normalizeID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "node-modules" || id == "nodemodules" {
		return "node_modules"
	}
	return id
}

func containsID(list []string, id string) bool {
	for _, v := range list {
		if normalizeID(v) == id {
			return true
		}
	}
	return false
}

func newRoot(d Deps) *cobra.Command {
	sc := &scanCmd{d: d}
	root := &cobra.Command{
		Use:   "diskdiet",
		Short: "See where your developer disk space went, and get it back safely",
		Long: `diskdiet measures the caches developer tools leave behind (npm, Yarn, pnpm,
Bun, pip, uv, Cargo, Go, Gradle, Maven, Playwright, Cypress, Xcode, Docker) and
the node_modules and target folders of projects you haven't touched, then frees
what is old: not used in 30 days by default.

Run without a command, diskdiet scans and changes nothing.`,
		Example: `  diskdiet                         # what's using space, and what can go
  diskdiet clean                   # free what's older than 30 days, after asking
  diskdiet clean --older-than 7d --only npm,docker
  diskdiet clean --all --dry-run   # what removing everything would free`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          sc.run,
	}
	sc.register(root)
	root.AddCommand(newScan(d), newClean(d), newMCP(d), newVersion(d))
	root.CompletionOptions.HiddenDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &exitError{ExitUsage, err} })
	return root
}

func newVersion(d Deps) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the diskdiet version",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			info := buildinfo.Get()
			if asJSON {
				return writeJSON(d.Out, info)
			}
			fmt.Fprintln(d.Out, info.String())
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

// scanAll scans sources concurrently, keeping their order.
func scanAll(ctx context.Context, d Deps, srcs []source.Source, o source.Options) []*source.Report {
	if d.Progress {
		names := make([]string, len(srcs))
		for i, s := range srcs {
			names[i] = s.Name()
		}
		fmt.Fprintf(d.Err, "Scanning %s…", strings.Join(names, ", "))
		defer fmt.Fprint(d.Err, "\r\033[K")
	}
	reports := make([]*source.Report, len(srcs))
	var wg sync.WaitGroup
	for i, s := range srcs {
		wg.Go(func() { reports[i] = s.Scan(ctx, o) })
	}
	wg.Wait()
	return reports
}
