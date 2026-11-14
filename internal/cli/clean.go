package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/roystondwork-dev/diskdiet/internal/source"
	"github.com/roystondwork-dev/diskdiet/internal/units"
)

const defaultAge = 30 * 24 * time.Hour

type cleanCmd struct {
	common
	d       Deps
	yes     bool
	dryRun  bool
	volumes bool
	force   bool
	keep    []string
	trash   bool
}

func newClean(d Deps) *cobra.Command {
	cc := &cleanCmd{d: d}
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Free space: remove what's old, after showing the list and asking",
		Long: `clean removes what diskdiet scan reports as old enough:

  npm, Yarn, Bun     cached packages not changed in --older-than (default 30d)
  pnpm               packages no project uses (pnpm store prune), and old metadata
  Docker             unused images, stopped containers and build cache older than
                     --older-than; unused volumes only with --volumes
  pip, Cargo, Go     cached downloads and builds not changed in --older-than
  Gradle, Maven      downloaded Gradle versions and caches; Maven artifacts
  uv                 the whole cache (uv cache clean), only with --all
  Playwright,        older browser and app versions; the newest always stays
  Cypress
  Xcode              DerivedData and device support files (never Archives)
  node_modules,      in projects where nothing has changed for --older-than
  target (Rust, Maven)

Everything removed from a cache is downloaded again when a project needs it.
It shows what it will remove and asks first, unless --yes is given.`,
		Example: `  diskdiet clean
  diskdiet clean --older-than 7d --only npm
  diskdiet clean --all --skip node_modules --yes`,
		Args: cobra.NoArgs,
		RunE: cc.run,
	}
	cc.register(cmd)
	f := cmd.Flags()
	f.BoolVarP(&cc.yes, "yes", "y", false, "don't ask before removing")
	f.BoolVarP(&cc.dryRun, "dry-run", "n", false, "show what would be removed, and remove nothing")
	f.BoolVar(&cc.volumes, "volumes", false, "also remove unused Docker volumes (they can hold databases)")
	f.BoolVar(&cc.force, "force", false, "clean even while a package manager is installing")
	f.BoolVar(&cc.trash, "trash", false, "move node_modules and target folders to the Trash instead of deleting them (space is freed when the Trash is emptied)")
	f.StringArrayVar(&cc.keep, "keep", nil, "leave this node_modules or target folder alone (repeatable)")
	return cmd
}

type cleanJSON struct {
	SchemaVersion int              `json:"schemaVersion"`
	OlderThan     string           `json:"olderThan"`
	DryRun        bool             `json:"dryRun"`
	Plan          []sourceJSON     `json:"plan"`
	Results       []*source.Result `json:"results"`
	FreedBytes    int64            `json:"freedBytes"`
	TrashedBytes  int64            `json:"trashedBytes,omitempty"`
}

func (cc *cleanCmd) run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	now := cc.d.Now()
	o, err := cc.options(now)
	if err != nil {
		return err
	}
	o.Volumes = cc.volumes
	o.Trash = cc.trash
	if cc.json && !cc.yes && !cc.dryRun {
		return &exitError{ExitUsage, errors.New("--json needs --yes or --dry-run, since diskdiet can't ask")}
	}
	all, home, err := cc.d.Sources()
	if err != nil {
		return err
	}
	srcs, err := cc.selected(all)
	if err != nil {
		return err
	}
	if !cc.yes && !cc.dryRun && !cc.d.Interactive {
		return &exitError{ExitUsage, errors.New("not asking without a terminal; add --yes to clean, or --dry-run to preview")}
	}
	reports := scanAll(ctx, cc.d, srcs, o)
	if len(cc.keep) > 0 {
		keep := make([]string, 0, len(cc.keep))
		for _, k := range cc.keep {
			abs, err := absPath(k)
			if err != nil {
				return &exitError{ExitUsage, err}
			}
			keep = append(keep, abs)
		}
		for _, r := range reports {
			if source.IsProjects(r.ID) {
				source.KeepProjects(r, keep)
			}
		}
	}
	var todo []int
	var planned int64
	for i, r := range reports {
		if r.PrunableBytes > 0 || r.PrunableItems > 0 {
			todo = append(todo, i)
			planned += r.PrunableBytes
		}
	}
	out := cc.d.Out
	if cc.json && (cc.dryRun || len(todo) == 0) {
		return writeJSON(out, cleanJSON{SchemaVersion: SchemaVersion, OlderThan: ageLabel(o), DryRun: cc.dryRun,
			Plan: withProjects(pick(reports, todo), now), Results: []*source.Result{}})
	}
	if len(todo) == 0 {
		fmt.Fprintln(out, "Nothing to free"+ageSuffix(o)+".")
		return nil
	}
	if !cc.json {
		renderPlan(out, reports, todo, o, home, now)
	}
	if cc.dryRun {
		fmt.Fprintln(out, "\nDry run: nothing was removed.")
		return nil
	}
	if !cc.yes {
		fmt.Fprintf(out, "\nFree about %s? [y/N] ", units.FormatBytes(planned))
		if !confirm(cc.d.In) {
			fmt.Fprintln(out, "Nothing was removed.")
			return nil
		}
	}

	busy := map[string]string{}
	if !cc.force {
		if b, err := cc.d.Busy(ctx); err == nil {
			busy = b
		}
	}
	var results []*source.Result
	var freed, trashed int64
	failed := false
	for _, i := range todo {
		s, r := srcs[i], reports[i]
		if what, ok := busy[s.ID()]; ok {
			results = append(results, &source.Result{ID: s.ID(), Name: s.Name(),
				Skipped: fmt.Sprintf("%q is running; try again when it finishes, or add --force", what)})
			continue
		}
		if !cc.json && cc.d.Progress {
			fmt.Fprintf(cc.d.Err, "Cleaning %s…", s.Name())
		}
		res := s.Clean(ctx, r, o)
		if !cc.json && cc.d.Progress {
			fmt.Fprint(cc.d.Err, "\r\033[K")
		}
		results = append(results, res)
		freed += res.Freed
		trashed += res.Trashed
		if len(res.Errors) > 0 {
			failed = true
		}
		if ctx.Err() != nil {
			break
		}
	}
	if cc.json {
		if err := writeJSON(out, cleanJSON{SchemaVersion: SchemaVersion, OlderThan: ageLabel(o),
			Plan: withProjects(pick(reports, todo), now), Results: results, FreedBytes: freed, TrashedBytes: trashed}); err != nil {
			return err
		}
	} else {
		renderResults(out, results, freed, trashed)
	}
	if failed {
		return &exitError{code: ExitPartial, err: errors.New("some items could not be removed")}
	}
	return nil
}

func ageSuffix(o source.Options) string {
	if o.Age <= 0 {
		return ""
	}
	return " older than " + units.FormatAge(o.Age)
}

func pick(reports []*source.Report, idx []int) []*source.Report {
	out := make([]*source.Report, 0, len(idx))
	for _, i := range idx {
		out = append(out, reports[i])
	}
	return out
}

func confirm(in io.Reader) bool {
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// maxProjects is how many node_modules folders the preview lists.
const maxProjects = 10

func renderPlan(w io.Writer, reports []*source.Report, todo []int, o source.Options, home string, now time.Time) {
	if o.Age > 0 {
		fmt.Fprintf(w, "diskdiet will remove (older than %s):\n\n", units.FormatAge(o.Age))
	} else {
		fmt.Fprint(w, "diskdiet will remove:\n\n")
	}
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, i := range todo {
		r := reports[i]
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.Name, prunableLabel(r), planDetail(r, o))
	}
	_ = tw.Flush()
	for _, i := range todo {
		r := reports[i]
		if !source.IsProjects(r.ID) {
			continue
		}
		ps := source.Projects(r, now)
		fmt.Fprintf(w, "\n  %s, biggest first:\n", r.Name)
		ptw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		for j, p := range ps {
			if j == maxProjects {
				fmt.Fprintf(ptw, "    … and %d more\t\t\n", len(ps)-maxProjects)
				break
			}
			fmt.Fprintf(ptw, "    %s\t%s\tunchanged for %s\n", tildify(p.Path, home), units.FormatBytes(p.Bytes), idle(p.Idle))
		}
		_ = ptw.Flush()
	}
	for _, i := range todo {
		for _, n := range reports[i].Notes {
			fmt.Fprintf(w, "  • %s: %s\n", reports[i].Name, n)
		}
	}
}

func planDetail(r *source.Report, o source.Options) string {
	switch r.ID {
	case "docker":
		s := "unused images, stopped containers, build cache"
		if o.Volumes {
			s += ", unused volumes"
		}
		return s
	case "pnpm":
		return "packages no project uses"
	case "uv":
		return "the whole cache (uv cache clean)"
	case "playwright", "cypress":
		return "older versions; the newest stays"
	case "node_modules", "target":
		if o.Trash {
			return "projects you haven't touched, to the Trash"
		}
		return "projects you haven't touched"
	default:
		return fmt.Sprintf("%d cached items", r.PrunableItems)
	}
}

func idle(d time.Duration) string {
	days := int(d.Hours() / 24)
	switch {
	case days >= 365:
		return fmt.Sprintf("%.1f years", float64(days)/365)
	case days >= 60:
		return fmt.Sprintf("%d months", days/30)
	case days == 0:
		return "less than a day"
	case days == 1:
		return "1 day"
	default:
		return fmt.Sprintf("%d days", days)
	}
}

func renderResults(w io.Writer, results []*source.Result, freed, trashed int64) {
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "WHAT\tFREED\tRESULT")
	for _, r := range results {
		status := "done"
		if r.Trashed > 0 {
			status = "moved to the Trash"
		}
		switch {
		case r.Skipped != "":
			status = "skipped: " + r.Skipped
		case len(r.Errors) > 0:
			status = fmt.Sprintf("%d could not be removed", len(r.Errors))
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, units.FormatBytes(r.Freed+r.Trashed), status)
	}
	_ = tw.Flush()
	for _, r := range results {
		for _, n := range r.Notes {
			fmt.Fprintf(w, "  • %s: %s\n", r.Name, n)
		}
		for j, e := range r.Errors {
			if j == 3 {
				fmt.Fprintf(w, "  ! %s: … and %d more\n", r.Name, len(r.Errors)-3)
				break
			}
			fmt.Fprintf(w, "  ! %s: %s\n", r.Name, e)
		}
	}
	fmt.Fprintf(w, "\nFreed %s.\n", units.FormatBytes(freed))
	if trashed > 0 {
		fmt.Fprintf(w, "Moved %s to the Trash: empty it to free that space too, or put folders back from it.\n", units.FormatBytes(trashed))
	}
}
