//go:build !windows

package system

import (
	"context"
	"errors"
	"syscall"
)

func waitForParentExit(ctx context.Context, pid int) error {
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return err
		}
		if err := waitForParentPoll(ctx); err != nil {
			return err
		}
	}
}
