package updates

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationCheckLeavesExistingApplicationUntouched(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "res-downloader.AppImage")
	// Replacing an executable needs a writable directory, not a writable file.
	if err := os.WriteFile(target, []byte("previous application"), 0555); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"binary", "appimage"} {
		if err := checkInstallationTarget(target, kind); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != "previous application" {
		t.Fatalf("permission check changed existing application: %q, %v", raw, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("permission check left temporary files: %v, %v", entries, err)
	}
}

func TestInstallationPermissionLossPreventsDownloadAndHandoff(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to directories regardless of permission bits")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "res-downloader.AppImage")
	if err := os.WriteFile(target, []byte("previous application"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	for _, kind := range []string{"binary", "appimage"} {
		for _, state := range []string{"available", "ready"} {
			t.Run(kind+"/"+state, func(t *testing.T) {
				m := &Manager{
					target: target, kind: kind, root: filepath.Join(dir, "updates"),
					status: Status{State: state, Available: true, CanInstall: true},
				}
				var err error
				if state == "ready" {
					err = m.Prepare()
				} else {
					err = m.Download(false)
				}
				if !errors.Is(err, os.ErrPermission) {
					t.Fatalf("expected a permission error before starting work, got %v", err)
				}
				if m.status.CanInstall || m.status.State != "error" || m.status.Error == "" || m.job != nil || m.done != nil {
					t.Fatalf("permission failure did not prevent automatic update: %+v", m.status)
				}
				if _, err := os.Stat(m.root); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("permission check created update files: %v", err)
				}
			})
		}
	}
	// Debian packages use pkexec and must not require an unprivileged write.
	if err := checkInstallationTarget(target, "deb"); err != nil {
		t.Fatalf("permission probe blocked an elevated package installation: %v", err)
	}
}

func TestAppImageInstallationResolvesExternalSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "res-downloader.AppImage")
	if err := os.WriteFile(target, []byte("previous application"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "launcher")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPIMAGE", link)
	got, kind := installationTarget()
	if got != target || kind != "appimage" {
		t.Fatalf("installation targets launcher instead of image: %q, %q", got, kind)
	}
}
