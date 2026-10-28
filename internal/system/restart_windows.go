package system

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

func waitForParentExit(ctx context.Context, pid int) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil // The parent already exited.
	}
	if err != nil {
		return fmt.Errorf("open previous application process: %w", err)
	}
	defer windows.CloseHandle(handle)
	for {
		state, err := windows.WaitForSingleObject(handle, 0)
		if err != nil {
			return err
		}
		if state == windows.WAIT_OBJECT_0 {
			return nil
		}
		if err := waitForParentPoll(ctx); err != nil {
			return err
		}
	}
}
