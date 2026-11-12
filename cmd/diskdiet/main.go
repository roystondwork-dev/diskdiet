// Command diskdiet shows where developer disk space went and frees it safely.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/roystondwork-dev/diskdiet/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Execute(ctx, cli.SystemDeps(), os.Args[1:])
	stop()
	os.Exit(code)
}
