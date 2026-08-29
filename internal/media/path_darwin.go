//go:build darwin

package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const loginPathMarker = "__RES_DOWNLOADER_PATH__"

var loginPathCache struct {
	sync.Once
	directories []string
}

func platformLookPath(name string) (string, error) {
	directories := append([]string{}, darwinLoginPathDirectories()...)
	directories = append(directories, "/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin")
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		directories = append(directories, filepath.Join(home, ".local", "bin"))
	}
	for _, directory := range directories {
		if directory = strings.TrimSpace(directory); directory == "" {
			continue
		}
		candidate := filepath.Join(directory, name)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func darwinLoginPathDirectories() []string {
	loginPathCache.Do(func() {
		shell := strings.TrimSpace(os.Getenv("SHELL"))
		if shell == "" {
			shell = "/bin/zsh"
		}
		if !filepath.IsAbs(shell) {
			return
		}
		if info, err := os.Stat(shell); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, shell, "-lc", `printf '\n__RES_DOWNLOADER_PATH__%s' "$PATH"`)
		command.WaitDelay = 250 * time.Millisecond
		output, err := command.CombinedOutput()
		if err != nil {
			return
		}
		text := string(output)
		markerIndex := strings.LastIndex(text, loginPathMarker)
		if markerIndex < 0 {
			return
		}
		pathValue := strings.TrimSpace(text[markerIndex+len(loginPathMarker):])
		loginPathCache.directories = filepath.SplitList(pathValue)
	})
	return loginPathCache.directories
}
