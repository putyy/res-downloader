//go:build windows

package media

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func configureMediaCommand(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.HideWindow = true
	command.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
