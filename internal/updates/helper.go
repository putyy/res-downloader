package updates

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Job struct {
	Version string `json:"version"`
	Parent  int    `json:"parent"`
	File    string `json:"file"`
	Digest  string `json:"digest"`
	Target  string `json:"target"`
	Kind    string `json:"kind"`
	Gate    string `json:"gate"`
	Root    string `json:"root"`
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func prepareHelper(root, file, digest, target, kind, version string) (*Job, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	folder, err := os.MkdirTemp(root, "helper-")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(folder)
		}
	}()
	helper := filepath.Join(folder, "updater"+filepath.Ext(executable))
	if err := copyFile(executable, helper, 0700); err != nil {
		return nil, err
	}
	job := &Job{Version: version, Parent: os.Getpid(), File: file, Digest: digest, Target: target, Kind: kind, Gate: filepath.Join(folder, "continue"), Root: root}
	raw, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	jobFile := filepath.Join(folder, "job.json")
	if err := os.WriteFile(jobFile, raw, 0600); err != nil {
		return nil, err
	}
	cmd := exec.Command(helper, "--apply-update", jobFile)
	detachHelper(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-exited:
			return nil, fmt.Errorf("update helper did not start: %v", err)
		case <-timer.C:
			_ = cmd.Process.Kill()
			<-exited
			return nil, errors.New("update helper startup timed out")
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(folder, "helper-ready")); err == nil {
				success = true
				return job, nil
			}
		}
	}
}

// RunHelper must run before Wails and before opening any application databases.
func RunHelper(jobFile string) error {
	raw, err := os.ReadFile(jobFile)
	if err != nil {
		return err
	}
	if len(raw) > 16384 {
		return errors.New("invalid update job")
	}
	var job Job
	if err := json.Unmarshal(raw, &job); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	folder := filepath.Dir(executable)
	if filepath.Dir(jobFile) != folder || job.Gate != filepath.Join(folder, "continue") || filepath.Dir(folder) != job.Root || job.Parent <= 0 || !filepath.IsAbs(job.Target) || !filepath.IsAbs(job.File) {
		return errors.New("invalid helper paths")
	}
	if err := os.WriteFile(filepath.Join(folder, "helper-ready"), []byte("ready"), 0600); err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		_, gateErr := os.Stat(job.Gate)
		if gateErr == nil && !processRunning(job.Parent) {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("application did not finish shutting down; update was not installed")
		}
		time.Sleep(200 * time.Millisecond)
	}
	file, err := os.Open(job.File)
	if err == nil {
		hash := sha256.New()
		_, err = io.Copy(hash, file)
		file.Close()
		if err == nil && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), job.Digest) {
			err = errors.New("downloaded update changed before installation")
		}
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(job.Root, "expected-version.txt"), []byte(job.Version), 0600)
	}
	if err == nil {
		err = install(job)
	}
	if err != nil {
		_ = os.WriteFile(filepath.Join(job.Root, "last-error.txt"), []byte("Update failed; the previous application was kept where possible. "+err.Error()), 0600)
	}
	// The helper is never elevated; the restarted application retains the user's identity.
	if startErr := restart(job); startErr != nil {
		_ = os.WriteFile(filepath.Join(job.Root, "last-error.txt"), []byte("Could not restart automatically. Please reopen the application. "+startErr.Error()), 0600)
		return startErr
	}
	if err == nil {
		_ = os.RemoveAll(filepath.Dir(job.File))
	}
	return err
}
