package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// RelaunchCommand defers desktop startup until the previous instance exits.
// Callers start it only after their cleanup and configuration changes finish.
func RelaunchCommand(executable string) *exec.Cmd {
	args := []string{"--wait-for-parent", strconv.Itoa(os.Getpid())}
	if image := os.Getenv("APPIMAGE"); runtime.GOOS == "linux" && filepath.IsAbs(image) {
		return AppImageCommand(image, args...)
	}
	return exec.Command(executable, args...)
}

func WaitForParentExit(ctx context.Context, pid int) error {
	if pid <= 0 || pid == os.Getpid() {
		return fmt.Errorf("invalid restart process ID %d", pid)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return waitForParentExit(ctx, pid)
}

func waitForParentPoll(ctx context.Context) error {
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting for previous application to exit: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
