package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	var err error
	if len(os.Args) < 2 {
		err = fmt.Errorf("usage: githubctl <version|extensions> [--output path]")
	} else {
		switch os.Args[1] {
		case "extensions":
			err = run(context.Background(), os.Args[2:], os.Stdout, os.Stderr)
		case "version":
			err = runVersion(context.Background(), os.Args[2:], os.Stdout, os.Stderr)
		default:
			err = fmt.Errorf("unknown command %q", os.Args[1])
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
