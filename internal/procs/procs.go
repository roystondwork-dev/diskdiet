// Package procs finds package managers that are installing right now, so their
// caches are not cleaned under them.
package procs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// Proc is a running process.
type Proc struct {
	PID     int32
	Name    string
	Cmdline []string
}

// List returns the running processes. Replaced in tests.
var List = func(ctx context.Context) ([]Proc, error) {
	ps, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	out := make([]Proc, 0, len(ps))
	for _, p := range ps {
		name, err := p.NameWithContext(ctx)
		if err != nil {
			continue
		}
		base := normalize(name)
		if base != "node" && managerOf(base) == "" {
			continue
		}
		args, _ := p.CmdlineSliceWithContext(ctx)
		out = append(out, Proc{PID: p.Pid, Name: name, Cmdline: args})
	}
	return out, nil
}

func normalize(name string) string {
	name = strings.ToLower(filepath.Base(name))
	return strings.TrimSuffix(name, ".exe")
}

// managerOf maps a program name to the source whose cache it uses.
func managerOf(base string) string {
	switch base {
	case "npm", "npx", "npm-cli.js", "npx-cli.js":
		return "npm"
	case "pnpm", "pnpx", "pnpm.cjs", "pnpm.js":
		return "pnpm"
	case "yarn", "yarn.js", "yarn.cjs":
		return "yarn"
	case "bun", "bunx":
		return "bun"
	}
	if strings.HasPrefix(base, "yarn-") && strings.HasSuffix(base, ".cjs") {
		return "yarn"
	}
	return ""
}

// installVerbs are subcommands that read or write a package cache. A
// long-running `npm run dev` or `bun server.ts` does not block cleaning.
var installVerbs = map[string]bool{
	"install": true, "i": true, "ci": true, "add": true, "update": true, "up": true,
	"upgrade": true, "remove": true, "rm": true, "uninstall": true, "dedupe": true,
	"exec": true, "x": true, "dlx": true, "create": true, "init": true, "fetch": true,
	"rebuild": true, "cache": true, "store": true, "pack": true, "publish": true,
}

// Busy returns, for each source ID whose package manager is installing, a
// description of the process.
func Busy(ctx context.Context) (map[string]string, error) {
	ps, err := List(ctx)
	if err != nil {
		return nil, err
	}
	busy := map[string]string{}
	for _, p := range ps {
		if id, what := classify(p); id != "" {
			if _, ok := busy[id]; !ok {
				busy[id] = what
			}
		}
	}
	return busy, nil
}

// classify decides whether p is a package manager at work on its cache.
func classify(p Proc) (id, what string) {
	args := p.Cmdline
	if len(args) == 0 {
		args = []string{p.Name}
	}
	// Find the package manager: the program itself, or the script node runs.
	i := 0
	id = managerOf(normalize(args[0]))
	if id == "" {
		id = managerOf(normalize(p.Name))
	}
	if normalize(p.Name) == "node" || normalize(args[0]) == "node" {
		id = ""
		for j := 1; j < len(args); j++ {
			if m := managerOf(normalize(args[j])); m != "" {
				id, i = m, j
				break
			}
		}
	}
	if id == "" {
		return "", ""
	}
	prog := normalize(args[i])
	desc := strings.Join(append([]string{prog}, args[i+1:]...), " ")
	// npx, pnpx and bunx run packages straight from the cache.
	if prog == "npx" || prog == "npx-cli.js" || prog == "pnpx" || prog == "bunx" {
		return id, desc
	}
	// Bare `yarn` installs.
	if (prog == "yarn" || strings.HasPrefix(prog, "yarn")) && len(args) == i+1 {
		return id, desc
	}
	for _, a := range args[i+1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if installVerbs[a] {
			return id, desc
		}
		break // the first plain word is the subcommand
	}
	return "", ""
}
