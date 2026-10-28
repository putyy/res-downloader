// pluginctl exposes the existing plugin authoring commands without the desktop UI.
package main

import (
	"fmt"
	"os"

	"res-downloader/internal/plugin"
)

func main() {
	if err := plugin.RunPluginCLI(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "plugin command failed:", err)
		os.Exit(1)
	}
}
