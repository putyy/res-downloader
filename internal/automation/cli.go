package automation

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"res-downloader/internal/control"
	"time"
)

const usage = `Control the running res-downloader desktop application.

Usage:
  res-downloader cli resources list [--offset 0] [--limit 100] [--json]
  res-downloader cli downloads create --resource-id ID [--json]
  res-downloader cli downloads list [--json]
  res-downloader cli downloads get --id ID [--json]
  res-downloader cli downloads pause|resume|cancel|retry --id ID [--json]
  res-downloader mcp --stdio

Options (after the command):
  --session-file PATH  Override local session discovery file
  --timeout 15s        Per-request timeout (maximum 5m)
  --json              JSON output (default; accepted for scripting)

Exit codes: 0 success, 1 operation/protocol failure, 2 invalid arguments,
            3 desktop unavailable or local session expired.
Downloads continue in the desktop app after a command or MCP process exits.
`

// Run handles only automation commands; callers dispatch before desktop startup.
// stdin must be closable to allow MCP cancellation to unblock a pending read.
func Run(ctx context.Context, args []string, stdin io.ReadCloser, stdout, stderr io.Writer, version string) int {
	err := run(ctx, args, stdin, stdout, version)
	if err == nil || errors.Is(err, context.Canceled) {
		return 0
	}
	kind, code := "operation_failed", 1
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		kind = commandErr.kind
		switch kind {
		case "invalid_arguments":
			code = 2
		case "unavailable", "unauthorized":
			code = 3
		}
	}
	_ = json.NewEncoder(stderr).Encode(map[string]any{"code": 0, "message": err.Error(), "errorCode": kind})
	return code
}

func run(ctx context.Context, args []string, stdin io.ReadCloser, stdout io.Writer, version string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || (len(args) == 1 && args[0] == "cli") {
		_, err := io.WriteString(stdout, usage)
		return err
	}
	if args[0] == "cli" && len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
		_, err := io.WriteString(stdout, usage)
		return err
	}
	flags := flag.NewFlagSet("automation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("session-file", control.DefaultSessionPath(), "local session file")
	timeout := flags.Duration("timeout", 15*time.Second, "request timeout")
	var op operation
	var offset, limit *int
	var id *string
	var remaining []string
	switch args[0] {
	case "mcp":
		flags.Bool("stdio", true, "use MCP stdio transport")
		remaining = args[1:]
	case "cli":
		if len(args) < 3 {
			return fail("invalid_arguments", "expected a resource or download command; use cli --help")
		}
		for _, candidate := range operations {
			if candidate.group == args[1] && candidate.command == args[2] {
				op = candidate
				break
			}
		}
		if op.name == "" {
			return fail("invalid_arguments", "unknown command; use cli --help")
		}
		flags.Bool("json", true, "JSON output")
		if op.argument == "page" {
			offset = flags.Int("offset", 0, "page offset")
			limit = flags.Int("limit", 100, "page size")
		} else if op.argument != "" {
			name := "id"
			if op.argument == "resourceId" {
				name = "resource-id"
			}
			id = flags.String(name, "", "resource or task ID")
		}
		remaining = args[3:]
	default:
		return fail("invalid_arguments", "expected cli or mcp; use --help")
	}
	if err := flags.Parse(remaining); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err = io.WriteString(stdout, usage)
			return err
		}
		return fail("invalid_arguments", err.Error())
	}
	if flags.NArg() != 0 {
		return fail("invalid_arguments", "unexpected positional arguments")
	}
	if *timeout <= 0 || *timeout > 5*time.Minute || *path == "" {
		return fail("invalid_arguments", "timeout must be > 0 and <= 5m; session-file must not be empty")
	}
	for _, name := range []string{"json", "stdio"} {
		if option := flags.Lookup(name); option != nil && option.Value.String() != "true" {
			return fail("invalid_arguments", fmt.Sprintf("--%s=false is not supported", name))
		}
	}
	c := newClient(*path, *timeout)
	if args[0] == "mcp" {
		return runMCP(ctx, c, stdin, stdout, version)
	}
	arguments := map[string]any{}
	if offset != nil {
		arguments["offset"], arguments["limit"] = *offset, *limit
	}
	if id != nil {
		arguments[op.argument] = *id
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		return err
	}
	result, err := c.call(ctx, op, raw)
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(result)
}
