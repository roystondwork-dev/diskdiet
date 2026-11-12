## diskdiet mcp

Serve diskdiet to AI assistants (Model Context Protocol)

### Synopsis

Run diskdiet as an MCP server on stdin/stdout, so AI coding assistants (Claude
Code, Cursor, VS Code Copilot, Windsurf, ...) can see what is using disk space
and free it when you ask.

Tools:
  scan_disk_usage   what developer tools use and what can be freed (changes nothing)
  free_disk_space   removes what the last scan showed, for the sources you choose

free_disk_space follows the same rules as 'diskdiet clean', and asks you to
confirm in the assistant when the assistant supports it.

Add it to your assistant (see docs/mcp.md for others):
  claude mcp add diskdiet -- diskdiet mcp

```
diskdiet mcp [flags]
```

### Examples

```
  claude mcp add diskdiet -- diskdiet mcp
  code --add-mcp '{"name":"diskdiet","command":"diskdiet","args":["mcp"]}'
```

### Options

```
  -h, --help   help for mcp
```

### SEE ALSO

* [diskdiet](diskdiet.md)	 - See where your developer disk space went, and get it back safely

