//go:build !darwin

package media

import "os/exec"

func platformLookPath(string) (string, error) {
	return "", exec.ErrNotFound
}
