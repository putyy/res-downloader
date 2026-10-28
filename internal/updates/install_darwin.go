package updates

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func installationTarget() (string, string) {
	executable, err := os.Executable()
	if err != nil {
		return "", ""
	}
	index := strings.LastIndex(executable, ".app/Contents/MacOS/")
	if index < 0 {
		return "", ""
	}
	target := executable[:index+4]
	if strings.HasPrefix(target, "/Volumes/") || strings.Contains(target, "/AppTranslocation/") {
		return "", ""
	}
	return target, "dmg"
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func install(job Job) error {
	if job.Kind != "dmg" {
		return errors.New("unsupported installation")
	}
	mount, err := os.MkdirTemp(filepath.Dir(job.File), "volume-")
	if err != nil {
		return err
	}
	defer os.Remove(mount)
	if raw, err := exec.Command("/usr/bin/hdiutil", "attach", "-readonly", "-nobrowse", "-mountpoint", mount, job.File).CombinedOutput(); err != nil {
		return fmt.Errorf("mount update: %w: %.1000s", err, raw)
	}
	defer exec.Command("/usr/bin/hdiutil", "detach", mount).Run()
	apps, err := filepath.Glob(filepath.Join(mount, "*.app"))
	if err != nil || len(apps) != 1 {
		return errors.New("update must contain exactly one application")
	}
	// Require the package identity to match before replacing the existing bundle.
	bundleID := func(app string) (string, error) {
		raw, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleIdentifier", filepath.Join(app, "Contents/Info.plist")).Output()
		return strings.TrimSpace(string(raw)), err
	}
	current, e1 := bundleID(job.Target)
	next, e2 := bundleID(apps[0])
	if e1 != nil || e2 != nil || current == "" || current != next {
		return errors.New("update application identity does not match")
	}
	// A unique sibling staging directory preserves the old app until replacement succeeds.
	script := "set -eu\nstage=$(/usr/bin/mktemp -d " + shellQuote(filepath.Join(filepath.Dir(job.Target), ".res-downloader-update.XXXXXX")) + ")\n" +
		"/usr/bin/ditto " + shellQuote(apps[0]) + " \"$stage/new.app\"\n" +
		"/bin/mv " + shellQuote(job.Target) + " \"$stage/previous.app\"\n" +
		"if /bin/mv \"$stage/new.app\" " + shellQuote(job.Target) + "; then /bin/rm -rf \"$stage\"; else /bin/mv \"$stage/previous.app\" " + shellQuote(job.Target) + "; exit 1; fi"
	// Prompt for elevation only when the installation directory isn't writable.
	probe, writeErr := os.CreateTemp(filepath.Dir(job.Target), ".res-downloader-permission-")
	var cmd *exec.Cmd
	if writeErr == nil {
		probe.Close()
		os.Remove(probe.Name())
		cmd = exec.Command("/bin/sh", "-c", script)
	} else {
		appleScript := "do shell script " + strconv.Quote(script) + " with administrator privileges"
		cmd = exec.Command("/usr/bin/osascript", "-e", appleScript)
	}
	if raw, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("replace application: %w: %.1500s", err, raw)
	}
	return nil
}
