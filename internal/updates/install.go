package updates

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Only portable Linux installations replace files without elevation. Probe the
// parent directory because installation stages and renames files there.
func checkInstallationTarget(target, kind string) error {
	if kind != "binary" && kind != "appimage" {
		return nil
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("locate installed application: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("installed application is not a regular file")
	}
	probe, err := os.MkdirTemp(filepath.Dir(target), ".res-downloader-permission-")
	if err != nil {
		return fmt.Errorf("installation directory is not writable; please update from the website: %w", err)
	}
	if err := os.Remove(probe); err != nil {
		return fmt.Errorf("clean installation permission check: %w", err)
	}
	return nil
}
