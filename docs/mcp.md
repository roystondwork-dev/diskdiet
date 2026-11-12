# diskdiet for AI assistants (MCP)

`diskdiet mcp` runs diskdiet as a [Model Context Protocol](https://modelcontextprotocol.io)
server, so an AI coding assistant can tell you what is using disk space and free it
when you ask. Ask things like *"my disk is almost full, what's taking space?"* or
*"clear old package caches, but keep Docker"*.

## Set up

diskdiet must be installed and on your `PATH` (see the [README](../README.md#install)).

**Claude Code**

```sh
claude mcp add diskdiet -- diskdiet mcp
```

**VS Code (Copilot)**

```sh
code --add-mcp '{"name":"diskdiet","command":"diskdiet","args":["mcp"]}'
```

**Cursor, Windsurf and others** use the same server definition in their MCP settings
file (for Cursor, `~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "diskdiet": { "command": "diskdiet", "args": ["mcp"] }
  }
}
```

## Tools

| Tool | What it does |
|---|---|
| `scan_disk_usage` | Measures every cache diskdiet knows and what is removable: things unused for `older_than_days` (default 30). Optional: `sources` to limit it, `projects` folders to search for `node_modules` and `target`, `include_docker_volumes`. **Changes nothing.** |
| `free_disk_space` | Removes what the last scan reported as removable, for the `sources` you choose. `keep_projects` leaves individual `node_modules` or `target` folders alone. |

## Safety

- **Nothing is removed without a scan you've seen.** `free_disk_space` only works
  within 15 minutes of a `scan_disk_usage`, removes exactly what that scan listed, and
  each scan is good for one clean.
- **You're asked.** The tool is marked destructive, so assistants ask your permission
  before calling it. When the assistant supports MCP confirmations (elicitation),
  diskdiet also asks you directly, for example *"Free about 12.3 GB from npm cache,
  Go caches?"*, and removes nothing unless you accept.
- **The same rules as `diskdiet clean`**: never outside a cache folder; the tool's own
  prune command for pnpm, uv and Docker; Docker volumes only when asked; a cache is
  skipped while its package manager is installing or building.
