//go:build !windows

package updates

import (
	"os"
	"os/exec"
	"syscall"
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
	} else {
		cmd = exec.Command(job.Target)
	}
	// AppImage launcher variables belong to the old mount and must not reach the replacement.
	for _, entry := range os.Environ() {
		keep := true
		for _, key := range []string{"APPIMAGE=", "APPDIR=", "ARGV0=", "OWD="} {
			if len(entry) >= len(key) && entry[:len(key)] == key {
				keep = false
			}
		}
		if keep {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	detachHelper(cmd)
	return cmd.Start()
}
