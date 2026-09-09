// Package control provides discovery and transport for local automation clients.
// It has no dependency on the desktop runtime or application databases.
package control

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/vrischmann/userdir"
)

type Session struct {
	Version int    `json:"version"`
	URL     string `json:"url"`
	Token   string `json:"token"`
}

func SessionPath(appDir string) string {
	return filepath.Join(appDir, "control", "session.json")
}

func DefaultSessionPath() string {
	return SessionPath(filepath.Join(userdir.GetConfigHome(), "res-downloader"))
}

func ReadSession(path string) (Session, error) {
	var session Session
	file, err := os.Open(path)
	if err != nil {
		return session, fmt.Errorf("cannot read local session; start the desktop application under the same user: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return session, errors.New("invalid local session file")
	}
	if err := json.NewDecoder(file).Decode(&session); err != nil {
		return session, errors.New("invalid local session; restart the desktop application")
	}
	parsed, err := url.Parse(session.URL)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Hostname() != "127.0.0.1" ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || session.Version != 1 || len(session.Token) != 64 {
		return Session{}, errors.New("invalid local session endpoint")
	}
	_, port, err := net.SplitHostPort(parsed.Host)
	value, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || value < 1 || value > 65535 {
		return Session{}, errors.New("invalid local session port")
	}
	if _, err := hex.DecodeString(session.Token); err != nil {
		return Session{}, errors.New("invalid local session credential")
	}
	return session, nil
}

func writeSession(path string, session Session) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := protectDirectory(dir); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".session-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := json.NewEncoder(file).Encode(session); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
