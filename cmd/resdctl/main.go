// resdctl is a console-only client. It controls the desktop app and does not
// start a headless capture or download service.
package main

import (
	"context"
	"os"
	"os/signal"
	"res-downloader/internal/automation"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	code := automation.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version)
	cancel()
	os.Exit(code)
}
