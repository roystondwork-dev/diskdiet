package source

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/roystondwork-dev/diskdiet/internal/units"
)

// dockerSource is Docker's images, containers, build cache and volumes.
//
// It drives the docker command line, so it works with whatever the current
// docker context points at: Docker Desktop, OrbStack, Colima, Rancher Desktop
// or a plain Docker Engine. Docker decides what is unused; diskdiet only asks
// it to prune, with an age filter.
type dockerSource struct{ env Env }

// NewDocker finds Docker.
func NewDocker(env Env) Source { return &dockerSource{env: env} }

func (s *dockerSource) ID() string   { return "docker" }
func (s *dockerSource) Name() string { return "Docker" }

type dfRow struct {
	Type        string `json:"Type"`
	TotalCount  string `json:"TotalCount"`
	Size        string `json:"Size"`
	Reclaimable string `json:"Reclaimable"`
}

type dockerPlan struct {
	desktop bool
	volumes bool
}

func (s *dockerSource) run(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.env.Run.Run(ctx, "docker", args...)
}

func (s *dockerSource) Scan(ctx context.Context, o Options) *Report {
	r := &Report{ID: s.ID(), Name: s.Name()}
	if _, err := s.env.Run.LookPath("docker"); err != nil {
		return r
	}
	osName, err := s.run(ctx, 20*time.Second, "info", "--format", "{{.OperatingSystem}}")
	if err != nil {
		r.Found = true
		r.Locations = []string{"docker"}
		r.Unavailable = true
		r.note("Docker is installed but not running; start it to see its disk use")
		return r
	}
	r.Found = true
	plan := &dockerPlan{desktop: strings.Contains(string(osName), "Docker Desktop"), volumes: o.Volumes}
	r.Locations = []string{"docker (" + strings.TrimSpace(string(osName)) + ")"}

	out, err := s.run(ctx, 2*time.Minute, "system", "df", "--format", "{{json .}}")
	if err != nil {
		r.Error = err.Error()
		return r
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		var row dfRow
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		size, _ := units.ParseDockerSize(row.Size)
		recl, _ := units.ParseDockerSize(row.Reclaimable)
		var n int
		_, _ = fmt.Sscanf(row.TotalCount, "%d", &n)
		r.Bytes += size
		r.Items += n
		volume := strings.Contains(strings.ToLower(row.Type), "volume")
		if volume && !o.Volumes {
			if recl > 0 {
				r.note(units.FormatBytes(recl) + " in unused volumes is kept; add --volumes to remove them (volumes can hold databases)")
			}
			continue
		}
		r.PrunableBytes += recl
	}
	if r.PrunableBytes > 0 {
		r.Estimate = true
		if o.Age > 0 {
			r.note("Docker removes unused images, stopped containers and build cache older than " + units.FormatAge(o.Age) + "; the size is the most it can free")
		}
	}
	r.plan = plan
	return r
}

var reclaimedRe = regexp.MustCompile(`(?i)total\s+reclaimed\s+space:\s*([0-9.]+\s*[a-z]+)`)

func (s *dockerSource) Clean(ctx context.Context, r *Report, o Options) *Result {
	res := &Result{ID: s.ID(), Name: s.Name()}
	plan, _ := r.plan.(*dockerPlan)
	if plan == nil {
		return res
	}
	var filter []string
	if o.Age > 0 {
		filter = []string{"--filter", "until=" + strconv.FormatInt(int64(o.Age/time.Hour), 10) + "h"}
		if o.Age < time.Hour {
			filter = []string{"--filter", "until=" + o.Age.String()}
		}
	}
	// Containers first, so images used only by removed containers can go too.
	cmds := [][]string{
		append([]string{"container", "prune", "--force"}, filter...),
		append([]string{"image", "prune", "--all", "--force"}, filter...),
		append([]string{"builder", "prune", "--all", "--force"}, filter...),
	}
	if plan.volumes {
		// Volumes have no age filter.
		cmds = append(cmds, []string{"volume", "prune", "--all", "--force"})
	}
	for _, args := range cmds {
		out, err := s.run(ctx, 30*time.Minute, args...)
		if err != nil {
			res.fail(err)
			continue
		}
		if m := reclaimedRe.FindSubmatch(out); m != nil {
			if n, err := units.ParseDockerSize(strings.ReplaceAll(string(m[1]), " ", "")); err == nil {
				res.Freed += n
			}
		}
		res.Removed += countDeleted(out)
	}
	if plan.desktop && res.Freed > 0 {
		switch s.env.GOOS {
		case "windows":
			res.Notes = append(res.Notes, "Docker Desktop's WSL disk file doesn't shrink by itself; see https://github.com/roystondwork-dev/diskdiet/blob/main/docs/docker.md")
		case "darwin":
			res.Notes = append(res.Notes, "Docker Desktop can take a few minutes to hand the space back to macOS")
		}
	}
	return res
}

// countDeleted counts the IDs docker lists as deleted.
func countDeleted(out []byte) int {
	n := 0
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "deleted: ") || (len(line) == 64 && isHex(line)) {
			n++
		}
	}
	return n
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
