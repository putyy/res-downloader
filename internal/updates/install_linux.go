package updates

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func installationTarget() (string, string) {
	if image := os.Getenv("APPIMAGE"); filepath.IsAbs(image) {
		return image, "appimage"
	}
	executable, err := os.Executable()
	if err != nil {
		return "", ""
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "dpkg-query", "-S", executable).Output(); err == nil && strings.HasPrefix(string(output), "res-downloader:") {
		if _, err := exec.LookPath("pkexec"); err == nil {
			return executable, "deb"
		}
		return "", ""
	}
	for _, prefix := range []string{"/usr/", "/opt/", "/snap/", "/nix/", "/var/lib/"} {
		if strings.HasPrefix(executable, prefix) {
			return "", ""
		}
	}
	return executable, "binary"
}
func install(job Job) error {
	if job.Kind == "deb" {
		dpkg, err := exec.LookPath("dpkg")
		if err != nil {
			return err
		}
		command := exec.Command("pkexec", dpkg, "--install", job.File)
		if raw, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("package installation: %w: %.1500s", err, raw)
		}
		return nil
	}
	if job.Kind != "binary" && job.Kind != "appimage" {
		return errors.New("unsupported installation")
	}
	// Stage beside the target to keep each rename on the same filesystem.
	stage, err := os.MkdirTemp(filepath.Dir(job.Target), ".res-downloader-update-")
	if err != nil {
		return err
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stage)
		}
	}()
	next := filepath.Join(stage, "new")
	backup := filepath.Join(stage, "previous")
	if err := copyFile(job.File, next, 0755); err != nil {
		return err
	}
	if err := os.Rename(job.Target, backup); err != nil {
		return err
	}
	if err := os.Rename(next, job.Target); err != nil {
		if restoreErr := os.Rename(backup, job.Target); restoreErr != nil {
			// Keep the backup if rollback failed.
			keepStage = true
			saved := backup
			return fmt.Errorf("install: %v; restore: %v (backup: %s)", err, restoreErr, saved)
		}
		return err
	}
	return nil
}
