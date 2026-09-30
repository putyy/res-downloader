//go:build !windows

package updates

import (
	"os/exec"
	"syscall"

	"res-downloader/internal/system"
)

func detachHelper(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func processRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
func restart(job Job) error {
	var cmd *exec.Cmd
	if job.Kind == "dmg" {
		cmd = exec.Command("/usr/bin/open", job.Target)
	} else if job.Kind == "appimage" {
		cmd = system.AppImageCommand(job.Target)
	} else {
		cmd = exec.Command(job.Target)
	}
	detachHelper(cmd)
	return cmd.Start()
}
