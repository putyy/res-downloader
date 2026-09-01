//go:build !windows

package media

import "os/exec"

func configureMediaCommand(*exec.Cmd) {}
