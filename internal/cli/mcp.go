package cli

import (
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/roystondwork-dev/diskdiet/internal/appcore"
	"github.com/roystondwork-dev/diskdiet/internal/buildinfo"
	"github.com/roystondwork-dev/diskdiet/internal/mcpserver"
)

func newMCP(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve diskdiet to AI assistants (Model Context Protocol)",
		Long: `Run diskdiet as an MCP server on stdin/stdout, so AI coding assistants (Claude
Code, Cursor, VS Code Copilot, Windsurf, ...) can see what is using disk space
and free it when you ask.

Tools:
  scan_disk_usage   what developer tools use and what can be freed (changes nothing)
  free_disk_space   removes what the last scan showed, for the sources you choose

free_disk_space follows the same rules as 'diskdiet clean', and asks you to
confirm in the assistant when the assistant supports it.

Add it to your assistant (see docs/mcp.md for others):
  claude mcp add diskdiet -- diskdiet mcp`,
		Example: `  claude mcp add diskdiet -- diskdiet mcp
  code --add-mcp '{"name":"diskdiet","command":"diskdiet","args":["mcp"]}'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			core := appcore.New(appcore.Deps{Sources: d.Sources, Busy: d.Busy, Now: d.Now})
			server := mcpserver.New(buildinfo.Get().Version, core)
			t := &mcp.IOTransport{Reader: io.NopCloser(d.In), Writer: nopWriteCloser{d.Out}}
			return server.Run(cmd.Context(), t) //nolint:wrapcheck // the SDK's errors are clear
		},
	}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
