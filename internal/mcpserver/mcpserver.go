// Package mcpserver serves diskdiet to AI assistants over the Model Context
// Protocol. It has two tools: scan_disk_usage, which changes nothing, and
// free_disk_space, which removes exactly what the last scan showed for the
// sources the user chose, after asking the user when the client can.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roystondwork-dev/diskdiet/internal/appcore"
	"github.com/roystondwork-dev/diskdiet/internal/source"
	"github.com/roystondwork-dev/diskdiet/internal/units"
)

const instructions = `diskdiet finds the disk space developer tools use: package caches (npm, Yarn,
pnpm, Bun, pip, uv, Cargo, Go, Gradle, Maven), browser test downloads (Playwright,
Cypress), Xcode, Docker, and the node_modules and target folders of projects
nobody has touched lately. When the user is low on disk space or asks what is
using it, call scan_disk_usage and summarize the biggest items. Only if the user
then wants to free space, call free_disk_space with the sources they chose. It
removes only what the scan reported as old, and everything removed from a cache
is downloaded again when a project needs it.`

// maxProjects is how many project folders a scan lists per source.
const maxProjects = 20

// ScanInput is scan_disk_usage's input.
type ScanInput struct {
	OlderThanDays *int     `json:"older_than_days,omitempty" jsonschema:"count things unused for this many days as removable (default 30; 0 counts everything)"`
	Sources       []string `json:"sources,omitempty" jsonschema:"only these sources (default all), e.g. [\"npm\", \"docker\"]"`
	Projects      []string `json:"projects,omitempty" jsonschema:"folders to search for node_modules and target folders (default: the user's home folder)"`
	DockerVolumes bool     `json:"include_docker_volumes,omitempty" jsonschema:"also count unused Docker volumes as removable; they can hold databases, so only when the user asks"`
}

// Project is a node_modules or target folder a clean would remove.
type Project struct {
	Path     string `json:"path"`
	Size     string `json:"size"`
	Bytes    int64  `json:"bytes"`
	IdleDays int    `json:"idle_days"`
}

// SourceOut is one source in a scan.
type SourceOut struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Size         string    `json:"size"`
	Bytes        int64     `json:"bytes"`
	CanFree      string    `json:"can_free"`
	CanFreeBytes int64     `json:"can_free_bytes"`
	Estimate     bool      `json:"estimate,omitempty" jsonschema:"can_free is the most that can be freed; the tool that owns the cache decides"`
	Unavailable  bool      `json:"unavailable,omitempty" jsonschema:"found but not measurable, e.g. Docker not running"`
	Removes      string    `json:"removes,omitempty" jsonschema:"what free_disk_space would remove from this source"`
	Locations    []string  `json:"locations"`
	Notes        []string  `json:"notes,omitempty"`
	Projects     []Project `json:"projects,omitempty" jsonschema:"the biggest project folders that would be removed; pass paths to keep in keep_projects"`
	MoreProjects int       `json:"more_projects,omitempty"`
}

// ScanOutput is scan_disk_usage's result.
type ScanOutput struct {
	Summary       string      `json:"summary"`
	OlderThanDays int         `json:"older_than_days"`
	Total         string      `json:"total"`
	TotalBytes    int64       `json:"total_bytes"`
	CanFree       string      `json:"can_free"`
	CanFreeBytes  int64       `json:"can_free_bytes"`
	Sources       []SourceOut `json:"sources"`
	NotFound      []string    `json:"not_found"`
	Next          string      `json:"next"`
}

// FreeInput is free_disk_space's input.
type FreeInput struct {
	Sources      []string `json:"sources" jsonschema:"the source ids to clean, from the last scan_disk_usage, e.g. [\"npm\", \"go\"]"`
	KeepProjects []string `json:"keep_projects,omitempty" jsonschema:"node_modules or target folder paths from the scan to leave alone"`
}

// ResultOut is what a clean did for one source.
type ResultOut struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Freed   string   `json:"freed"`
	Bytes   int64    `json:"freed_bytes"`
	Skipped string   `json:"skipped,omitempty"`
	Notes   []string `json:"notes,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

// FreeOutput is free_disk_space's result.
type FreeOutput struct {
	Summary    string      `json:"summary"`
	Freed      string      `json:"freed"`
	FreedBytes int64       `json:"freed_bytes"`
	Declined   bool        `json:"declined,omitempty" jsonschema:"the user said no when asked; nothing was removed"`
	Results    []ResultOut `json:"results"`
}

// server holds the scan between the two tools.
type server struct {
	core *appcore.Core
	last *appcore.Summary
}

func describe(id string, volumes bool) string {
	switch id {
	case "npm":
		return "cached packages and metadata, old npx installs and logs"
	case "yarn", "bun":
		return "cached packages"
	case "pnpm":
		return "packages no project uses (pnpm store prune) and old metadata"
	case "pip":
		return "cached downloads and built wheels"
	case "uv":
		return "the whole uv cache (uv cache clean)"
	case "cargo":
		return "downloaded crates, unpacked sources and git checkouts"
	case "go":
		return "build cache entries and module versions"
	case "gradle":
		return "downloaded Gradle versions and per-version caches"
	case "maven":
		return "artifact versions in ~/.m2"
	case "playwright":
		return "older browser versions (the newest of each stays)"
	case "cypress":
		return "older Cypress versions (the newest stays)"
	case "xcode":
		return "DerivedData and device support files (never Archives)"
	case "docker":
		if volumes {
			return "unused images, stopped containers, build cache and unused volumes"
		}
		return "unused images, stopped containers and build cache"
	case "node_modules", "target":
		return "the listed project folders; reinstall or rebuild brings them back"
	}
	return ""
}

func validSources(ids []string) error {
	known := source.IDs()
	for _, id := range ids {
		if !slices.Contains(known, id) {
			return fmt.Errorf("unknown source %q; choose from %s", id, strings.Join(known, ", "))
		}
	}
	return nil
}

func (s *server) scan(ctx context.Context, _ *mcp.CallToolRequest, in ScanInput) (*mcp.CallToolResult, ScanOutput, error) {
	days := 30
	if in.OlderThanDays != nil {
		days = *in.OlderThanDays
	}
	if days < 0 {
		return nil, ScanOutput{}, fmt.Errorf("older_than_days must be 0 or more, got %d", days)
	}
	if err := validSources(in.Sources); err != nil {
		return nil, ScanOutput{}, err
	}
	sum, err := s.core.Scan(ctx, appcore.ScanRequest{OlderThanDays: days, Volumes: in.DockerVolumes, Only: in.Sources, Projects: in.Projects}, nil)
	if err != nil {
		return nil, ScanOutput{}, fmt.Errorf("scanning: %w", err)
	}
	s.last = sum
	out := ScanOutput{
		OlderThanDays: days,
		Total:         units.FormatBytes(sum.Bytes), TotalBytes: sum.Bytes,
		CanFree: units.FormatBytes(sum.PrunableBytes), CanFreeBytes: sum.PrunableBytes,
		Sources: []SourceOut{}, NotFound: append([]string{}, sum.Missing...),
	}
	for _, v := range sum.Sources {
		so := SourceOut{
			ID: v.ID, Name: v.Name, Size: units.FormatBytes(v.Bytes), Bytes: v.Bytes,
			CanFree: units.FormatBytes(v.PrunableBytes), CanFreeBytes: v.PrunableBytes,
			Estimate: v.Estimate, Unavailable: v.Unavailable, Notes: v.Notes, Locations: []string{},
		}
		if v.PrunableBytes > 0 || v.PrunableItems > 0 {
			so.Removes = describe(v.ID, in.DockerVolumes)
		}
		for _, l := range v.Locations {
			so.Locations = append(so.Locations, firstNonEmpty(l.Path, l.Display))
		}
		for i, p := range v.Projects {
			if i == maxProjects {
				so.MoreProjects = len(v.Projects) - maxProjects
				break
			}
			so.Projects = append(so.Projects, Project{Path: p.Path, Size: units.FormatBytes(p.Bytes), Bytes: p.Bytes, IdleDays: p.IdleDays})
		}
		out.Sources = append(out.Sources, so)
	}
	age := fmt.Sprintf("unused for %d days", days)
	if days == 0 {
		age = "in total"
	}
	out.Summary = fmt.Sprintf("Developer tools use %s; %s %s can be freed.", out.Total, out.CanFree, age)
	if sum.PrunableBytes > 0 {
		out.Next = "Tell the user the biggest items. If they want to free space, call free_disk_space with the source ids they choose " +
			"(within 15 minutes of this scan). Ask before including node_modules, target or Docker."
	} else {
		out.Next = "Nothing is old enough to remove. A shorter older_than_days finds more."
	}
	return nil, out, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// confirmID names the confirmation input request.
const confirmID = "confirm_free"

// canConfirm reports whether the client can ask its user a question.
func canConfirm(req *mcp.CallToolRequest) bool {
	if req == nil || req.Session == nil {
		return false
	}
	params := req.Session.InitializeParams()
	return params != nil && params.Capabilities != nil && params.Capabilities.Elicitation != nil
}

// confirmation returns the user's answer to the confirmation, if this call is
// the retry that carries it.
func confirmation(req *mcp.CallToolRequest) (*mcp.ElicitResult, bool) {
	if req == nil || req.Params == nil {
		return nil, false
	}
	res, ok := req.Params.InputResponses[confirmID].(*mcp.ElicitResult)
	return res, ok
}

func (s *server) free(ctx context.Context, req *mcp.CallToolRequest, in FreeInput) (*mcp.CallToolResult, FreeOutput, error) {
	if len(in.Sources) == 0 {
		return nil, FreeOutput{}, errors.New("give at least one source id from scan_disk_usage")
	}
	if err := validSources(in.Sources); err != nil {
		return nil, FreeOutput{}, err
	}
	if s.last == nil {
		return nil, FreeOutput{}, errors.New("call scan_disk_usage first, show the user what can be freed, then call free_disk_space")
	}
	// What the chosen sources would free, for the confirmation.
	var planned int64
	var names []string
	keep := map[string]bool{}
	for _, p := range in.KeepProjects {
		keep[p] = true
	}
	for _, v := range s.last.Sources {
		if !slices.Contains(in.Sources, v.ID) || (v.PrunableBytes == 0 && v.PrunableItems == 0) {
			continue
		}
		b := v.PrunableBytes
		if source.IsProjects(v.ID) {
			b = 0
			for _, p := range v.Projects {
				if !keep[p.Path] {
					b += p.Bytes
				}
			}
		}
		planned += b
		names = append(names, v.Name)
	}
	if len(names) == 0 {
		return nil, FreeOutput{Summary: "Nothing to free in those sources.", Freed: "0 B", Results: []ResultOut{}}, nil
	}
	msg := fmt.Sprintf("Free about %s from %s? Removed cache entries download again when a project needs them.",
		units.FormatBytes(planned), strings.Join(names, ", "))
	if canConfirm(req) {
		answer, answered := confirmation(req)
		if !answered {
			// Ask the user; the client calls again with the answer. (For clients
			// on older protocol versions the SDK asks and calls back itself.)
			return &mcp.CallToolResult{InputRequests: mcp.InputRequestMap{confirmID: &mcp.ElicitParams{
				Message:         msg,
				RequestedSchema: &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{}},
			}}}, FreeOutput{}, nil
		}
		if answer.Action != "accept" {
			return nil, FreeOutput{Summary: "The user declined. Nothing was removed.", Freed: "0 B", Declined: true, Results: []ResultOut{}}, nil
		}
	}

	res, err := s.core.Clean(ctx, appcore.CleanRequest{IDs: in.Sources, KeepProjects: in.KeepProjects}, nil)
	s.last = nil // a scan is good for one clean
	if errors.Is(err, appcore.ErrNoScan) {
		return nil, FreeOutput{}, errors.New("the scan is older than 15 minutes; call scan_disk_usage again and confirm with the user")
	}
	if err != nil {
		return nil, FreeOutput{}, fmt.Errorf("cleaning: %w", err)
	}
	out := FreeOutput{Freed: units.FormatBytes(res.Freed), FreedBytes: res.Freed, Results: []ResultOut{}}
	problems := 0
	for _, r := range res.Results {
		out.Results = append(out.Results, ResultOut{ID: r.ID, Name: r.Name, Freed: units.FormatBytes(r.Freed), Bytes: r.Freed,
			Skipped: r.Skipped, Notes: r.Notes, Errors: r.Errors})
		if r.Skipped != "" || len(r.Errors) > 0 {
			problems++
		}
	}
	out.Summary = "Freed " + out.Freed + "."
	if problems > 0 {
		out.Summary += fmt.Sprintf(" %d source(s) were skipped or had items that couldn't be removed; see results.", problems)
	}
	return nil, out, nil
}

func boolPtr(b bool) *bool { return &b }

// freeSchema makes sources a required, non-empty array.
func freeSchema() *jsonschema.Schema {
	sc, err := jsonschema.For[FreeInput](nil)
	if err != nil {
		panic(err) // FreeInput is a fixed type
	}
	srcs := sc.Properties["sources"]
	srcs.Types, srcs.Type = nil, "array"
	one := 1
	srcs.MinItems = &one
	srcs.Items = &jsonschema.Schema{Type: "string", Enum: toAny(source.IDs())}
	return sc
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// New returns the MCP server; version is diskdiet's.
func New(version string, core *appcore.Core) *mcp.Server {
	srv := &server{core: core}
	s := mcp.NewServer(&mcp.Implementation{Name: "diskdiet", Title: "diskdiet", Version: version},
		&mcp.ServerOptions{Instructions: instructions})
	mcp.AddTool(s, &mcp.Tool{
		Name: "scan_disk_usage",
		Description: "Measure the disk space developer tools use on this machine and what can be freed: package caches " +
			"(npm, Yarn, pnpm, Bun, pip, uv, Cargo, Go, Gradle, Maven), Playwright and Cypress downloads, Xcode, Docker, and " +
			"node_modules and target folders of projects untouched for older_than_days. Changes nothing. " +
			"Use it when the user is low on disk space or asks what is using it.",
		Annotations: &mcp.ToolAnnotations{Title: "Scan disk usage", ReadOnlyHint: true, DestructiveHint: boolPtr(false),
			IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, srv.scan)
	mcp.AddTool(s, &mcp.Tool{
		Name: "free_disk_space",
		Description: "Remove what the last scan_disk_usage reported as removable, for the chosen sources only. " +
			"Call it only after showing the user the scan and getting their go-ahead; the user is also asked to confirm " +
			"when the client supports it. Cache entries download again when needed; node_modules come back with an install " +
			"and target folders with a build. It skips a cache while its package manager is installing.",
		InputSchema: freeSchema(),
		Annotations: &mcp.ToolAnnotations{Title: "Free disk space", DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)},
	}, srv.free)
	return s
}
