package updates

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func installationTarget() (string, string) {
	executable, err := os.Executable()
	if err != nil {
		return "", ""
	}
	if !strings.EqualFold(filepath.Base(executable), "res-downloader.exe") {
		return "", ""
	}
	return executable, "installer"
}
func detachHelper(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}
func processRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return err != windows.ERROR_INVALID_PARAMETER
	}
	defer windows.CloseHandle(handle)
	state, err := windows.WaitForSingleObject(handle, 0)
	return err != nil || state != windows.WAIT_OBJECT_0
}
func psQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
func install(job Job) error {
	if job.Kind != "installer" {
		return errors.New("unsupported installation")
	}
	// Keep a copy of the executable in case NSIS fails after replacing it.
	backup := filepath.Join(filepath.Dir(job.File), "previous.exe")
	_ = os.Remove(backup)
	if err := copyFile(job.Target, backup, 0600); err != nil {
		return err
	}
	// /D must be the final NSIS argument, without quotes, even when the directory has spaces.
	args := "/S /D=" + filepath.Dir(job.Target)
	script := "$ErrorActionPreference='Stop'; $p=Start-Process -FilePath " + psQuote(job.File) + " -ArgumentList " + psQuote(args) + " -Verb RunAs -Wait -PassThru; exit $p.ExitCode"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	detachHelper(cmd)
	if raw, err := cmd.CombinedOutput(); err != nil {
		// An unchanged executable needs no extra elevation prompt (for example UAC cancellation).
		if !sameFileContent(backup, job.Target) {
			restore := "$ErrorActionPreference='Stop'; Copy-Item -LiteralPath " + psQuote(backup) + " -Destination " + psQuote(job.Target) + " -Force"
			restoreCmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
				"$ErrorActionPreference='Stop'; $p=Start-Process powershell.exe -Verb RunAs -Wait -PassThru -ArgumentList "+psQuote("-NoProfile -NonInteractive -EncodedCommand "+encodePowerShell(restore))+"; exit $p.ExitCode")
			detachHelper(restoreCmd)
			if restoreErr := restoreCmd.Run(); restoreErr != nil {
				return fmt.Errorf("installer failed (%v); restore failed (%v); previous executable: %s", err, restoreErr, backup)
			}
		}
		return fmt.Errorf("installer failed or elevation was cancelled: %w: %.1500s", err, raw)
	}
	if _, err := os.Stat(job.Target); err != nil {
		return err
	}
	return nil
}
func restart(job Job) error {
	cmd := exec.Command(job.Target)
	cmd.Dir = filepath.Dir(job.Target)
	// The desktop app must not inherit the helper's HideWindow setting:
	// SW_HIDE overrides its first ShowWindow call, leaving it running unseen.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	return cmd.Start()
}

func sameFileContent(first, second string) bool {
	digest := func(path string) (string, error) {
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer file.Close()
		hash := sha256.New()
		_, err = io.Copy(hash, file)
		return string(hash.Sum(nil)), err
	}
	a, e1 := digest(first)
	b, e2 := digest(second)
	return e1 == nil && e2 == nil && a == b
}
func encodePowerShell(script string) string {
	words := utf16.Encode([]rune(script))
	raw := make([]byte, len(words)*2)
	for i, word := range words {
		binary.LittleEndian.PutUint16(raw[i*2:], word)
	}
	return base64.StdEncoding.EncodeToString(raw)
}
