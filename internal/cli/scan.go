package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/roystondwork-dev/diskdiet/internal/source"
	"github.com/roystondwork-dev/diskdiet/internal/units"
)

// SchemaVersion is the version of diskdiet's JSON output.
const SchemaVersion = 1

type scanCmd struct {
	common
	d Deps
}

func newScan(d Deps) *cobra.Command {
	sc := &scanCmd{d: d}
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Show what's using space and what can be freed (changes nothing)",
		Args:  cobra.NoArgs,
		RunE:  sc.run,
	}
	sc.register(cmd)
	return cmd
}

type scanJSON struct {
	SchemaVersion int              `json:"schemaVersion"`
	OlderThan     string           `json:"olderThan"`
	Sources       []*source.Report `json:"sources"`
	Bytes         int64            `json:"bytes"`
	PrunableBytes int64            `json:"prunableBytes"`
}

func (sc *scanCmd) run(cmd *cobra.Command, _ []string) error {
	o, err := sc.options(sc.d.Now())
	if err != nil {
		return err
	}
	all, home, err := sc.d.Sources()
	if err != nil {
		return err
	}
	srcs, err := sc.selected(all)
	if err != nil {
		return err
	}
	reports := scanAll(cmd.Context(), sc.d, srcs, o)
	if sc.json {
		return writeJSON(sc.d.Out, scanJSON{
			SchemaVersion: SchemaVersion,
			OlderThan:     ageLabel(o),
			Sources:       reports,
			Bytes:         totalBytes(reports),
			PrunableBytes: totalPrunable(reports),
		})
	}
	renderScan(sc.d.Out, reports, o, home)
	return nil
}

func ageLabel(o source.Options) string {
	if o.Age <= 0 {
		return "0"
	}
	return units.FormatAge(o.Age)
}

func totalBytes(rs []*source.Report) int64 {
	var n int64
	for _, r := range rs {
		n += r.Bytes
	}
	return n
}

func totalPrunable(rs []*source.Report) int64 {
	var n int64
	for _, r := range rs {
		n += r.PrunableBytes
	}
	return n
}

// renderScan prints the breakdown, biggest first.
func renderScan(w io.Writer, reports []*source.Report, o source.Options, home string) {
	found, missing := split(reports)
	sort.SliceStable(found, func(i, j int) bool { return found[i].Bytes > found[j].Bytes })

	if len(found) == 0 {
		fmt.Fprintln(w, "No caches found.")
	} else {
		tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		fmt.Fprintf(tw, "WHAT\tSIZE\tCAN FREE\tWHERE\n")
		for _, r := range found {
			size := units.FormatBytes(r.Bytes)
			if r.Unavailable {
				size = "–"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Name, size, prunableLabel(r), where(r, home))
		}
		_ = tw.Flush()
	}
	for _, r := range found {
		for _, n := range r.Notes {
			fmt.Fprintf(w, "  • %s: %s\n", r.Name, n)
		}
		if r.Error != "" {
			fmt.Fprintf(w, "  ! %s: %s\n", r.Name, r.Error)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(w, "Not found: %s\n", strings.Join(missing, ", "))
	}
	if len(found) == 0 {
		return
	}
	fmt.Fprintln(w)
	total, free := totalBytes(found), totalPrunable(found)
	what := "older than " + units.FormatAge(o.Age)
	if o.Age <= 0 {
		what = "in total"
	}
	if free == 0 {
		fmt.Fprintf(w, "Total %s. Nothing %s to free.\n", units.FormatBytes(total), what)
		return
	}
	fmt.Fprintf(w, "Total %s, of which about %s can be freed (%s).\n", units.FormatBytes(total), units.FormatBytes(free), what)
	fmt.Fprintf(w, "Run %s to free it; you'll see the list and be asked first.\n", cleanHint(o))
}

func cleanHint(o source.Options) string {
	switch {
	case o.Age <= 0:
		return "`diskdiet clean --all`"
	case o.Age == defaultAge:
		return "`diskdiet clean`"
	default:
		return "`diskdiet clean --older-than " + units.FormatAge(o.Age) + "`"
	}
}

func split(reports []*source.Report) (found []*source.Report, missing []string) {
	for _, r := range reports {
		if r.Found {
			found = append(found, r)
		} else {
			missing = append(missing, r.Name)
		}
	}
	return found, missing
}

func prunableLabel(r *source.Report) string {
	if r.PrunableBytes == 0 {
		return "–"
	}
	s := units.FormatBytes(r.PrunableBytes)
	if r.Estimate {
		s = "up to " + s
	}
	if source.IsProjects(r.ID) {
		s += fmt.Sprintf(" (%d of %d)", r.PrunableItems, r.Items)
	}
	return s
}

func where(r *source.Report, home string) string {
	locs := make([]string, len(r.Locations))
	for i, l := range r.Locations {
		locs[i] = tildify(l, home)
	}
	s := strings.Join(locs, ", ")
	if source.IsProjects(r.ID) {
		s = fmt.Sprintf("%d folders under %s", r.Items, s)
	}
	return s
}

// tildify shortens paths in the home folder to ~/….
func tildify(p, home string) string {
	if home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return "~" + string(filepath.Separator) + rel
	}
	return p
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("writing JSON: %w", err)
	}
	return nil
}

func absPath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding your home folder: %w", err)
		}
		p = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), string(filepath.Separator)))
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", p, err)
	}
	return abs, nil
}
