package procs

import (
	"context"
	"errors"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		p    Proc
		id   string
	}{
		{"npm install", Proc{Name: "npm", Cmdline: []string{"npm", "install"}}, "npm"},
		{"npm flags then ci", Proc{Name: "npm", Cmdline: []string{"/usr/local/bin/npm", "--silent", "ci"}}, "npm"},
		{"npm run dev", Proc{Name: "npm", Cmdline: []string{"npm", "run", "dev"}}, ""},
		{"node npm-cli install", Proc{Name: "node", Cmdline: []string{"node", "/usr/lib/node_modules/npm/bin/npm-cli.js", "install"}}, "npm"},
		{"node npm-cli run", Proc{Name: "node", Cmdline: []string{"node", "/x/npm-cli.js", "run", "build"}}, ""},
		{"node server", Proc{Name: "node", Cmdline: []string{"node", "server.js"}}, ""},
		{"npx", Proc{Name: "npx", Cmdline: []string{"npx", "serve"}}, "npm"},
		{"node npx-cli", Proc{Name: "node", Cmdline: []string{"node", "/x/npx-cli.js", "serve"}}, "npm"},
		{"bare yarn", Proc{Name: "yarn", Cmdline: []string{"yarn"}}, "yarn"},
		{"yarn release script", Proc{Name: "node", Cmdline: []string{"node", "/p/.yarn/releases/yarn-4.1.0.cjs"}}, "yarn"},
		{"yarn dev", Proc{Name: "yarn", Cmdline: []string{"yarn", "dev"}}, ""},
		{"bun run", Proc{Name: "bun", Cmdline: []string{"bun", "run", "dev"}}, ""},
		{"bun add", Proc{Name: "bun.exe", Cmdline: []string{`C:\bun\bun.exe`, "add", "react"}}, "bun"},
		{"bunx", Proc{Name: "bunx", Cmdline: []string{"bunx", "cowsay"}}, "bun"},
		{"pnpm dlx", Proc{Name: "pnpm", Cmdline: []string{"pnpm", "dlx", "create-vite"}}, "pnpm"},
		{"pnpx", Proc{Name: "pnpx", Cmdline: []string{"pnpx", "x"}}, "pnpm"},
		{"no cmdline", Proc{Name: "npm"}, ""},
		{"cmdline unknown, name known", Proc{Name: "pnpm", Cmdline: []string{"/weird/path", "install"}}, "pnpm"},
		{"unrelated", Proc{Name: "zsh", Cmdline: []string{"zsh"}}, ""},
	}
	for _, c := range cases {
		id, what := classify(c.p)
		if id != c.id {
			t.Errorf("%s: id = %q, want %q", c.name, id, c.id)
		}
		if id != "" && what == "" {
			t.Errorf("%s: empty description", c.name)
		}
	}
}

func TestBusy(t *testing.T) {
	orig := List
	t.Cleanup(func() { List = orig })
	List = func(context.Context) ([]Proc, error) {
		return []Proc{
			{Name: "npm", Cmdline: []string{"npm", "run", "dev"}},
			{Name: "npm", Cmdline: []string{"npm", "install", "react"}},
			{Name: "npm", Cmdline: []string{"npm", "ci"}},
			{Name: "bun", Cmdline: []string{"bun", "server.ts"}},
		}, nil
	}
	busy, err := Busy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(busy) != 1 || busy["npm"] != "npm install react" {
		t.Errorf("busy = %v", busy)
	}

	List = func(context.Context) ([]Proc, error) { return nil, errors.New("denied") }
	if _, err := Busy(context.Background()); err == nil {
		t.Error("want error")
	}
}

func TestListReal(t *testing.T) {
	// The real process list works and only returns candidates.
	ps, err := List(context.Background())
	if err != nil {
		t.Skip("process list unavailable:", err)
	}
	for _, p := range ps {
		if b := normalize(p.Name); b != "node" && managerOf(b) == "" {
			t.Errorf("unexpected process %q", p.Name)
		}
	}
}
