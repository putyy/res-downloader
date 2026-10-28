package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestWaitForParentExitRejectsInvalidParent(t *testing.T) {
	for _, pid := range []int{-1, 0, os.Getpid()} {
		if err := WaitForParentExit(context.Background(), pid); err == nil {
			t.Fatalf("accepted invalid parent %d", pid)
		}
	}
}

func TestWaitForParentExitHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WaitForParentExit(ctx, os.Getpid()+1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled restart wait returned %v", err)
	}
}

func TestRelaunchUsesExternalAppImageOnLinux(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "mount", "usr", "bin", "res-downloader")
	image := filepath.Join(dir, "res downloader.AppImage")
	for _, value := range []string{"", "relative.AppImage", image} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("APPIMAGE", value)
			cmd := RelaunchCommand(executable)
			want := executable
			if runtime.GOOS == "linux" && value == image {
				want = image
				if cmd.Dir != dir {
					t.Fatalf("relaunch inherited a potentially temporary directory: %q", cmd.Dir)
				}
			}
			args := []string{want, "--wait-for-parent", strconv.Itoa(os.Getpid())}
			if cmd.Path != want || !reflect.DeepEqual(cmd.Args, args) {
				t.Fatalf("relaunch command %q %q, want %q", cmd.Path, cmd.Args, args)
			}
		})
	}
}

func TestAppImageCommandDiscardsPreviousMountEnvironment(t *testing.T) {
	for _, key := range []string{"APPIMAGE", "APPDIR", "ARGV0", "OWD"} {
		t.Setenv(key, "/previous-image-mount")
	}
	t.Setenv("RES_DOWNLOADER_RESTART_TEST", "preserved")
	image := filepath.Join(t.TempDir(), "replacement.AppImage")
	cmd := AppImageCommand(image)
	if cmd.Path != image || cmd.Dir != filepath.Dir(image) {
		t.Fatalf("replacement command inherited previous image location: %q, %q", cmd.Path, cmd.Dir)
	}
	preserved := false
	for _, entry := range cmd.Env {
		key, value, _ := strings.Cut(entry, "=")
		switch key {
		case "APPIMAGE", "APPDIR", "ARGV0", "OWD":
			t.Fatalf("replacement inherited stale AppImage variable %s", key)
		case "RES_DOWNLOADER_RESTART_TEST":
			preserved = value == "preserved"
		}
	}
	if !preserved {
		t.Fatal("replacement discarded unrelated environment variables")
	}
}
