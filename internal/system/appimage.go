package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AppImageCommand starts the external image with a fresh mount, without
// inheriting the current image's mount paths or working directory.
func AppImageCommand(image string, args ...string) *exec.Cmd {
	cmd := exec.Command(image, args...)
	cmd.Dir = filepath.Dir(image)
	environment := os.Environ()
	cmd.Env = make([]string, 0, len(environment))
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "APPIMAGE", "APPDIR", "ARGV0", "OWD":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	return cmd
}
