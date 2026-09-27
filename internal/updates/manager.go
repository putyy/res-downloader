package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"res-downloader/internal/metadata"
	"res-downloader/internal/netproxy"
)

type Network struct {
	Port, Upstream string
	DownloadProxy  bool
}
type Status struct {
	State      string    `json:"state"`
	Manifest   *Manifest `json:"manifest,omitempty"`
	Available  bool      `json:"available"`
	CanInstall bool      `json:"canInstall"`
	Website    string    `json:"website"`
	Received   int64     `json:"received"`
	Total      int64     `json:"total"`
	Speed      int64     `json:"speed"`
	Slow       bool      `json:"slow"`
	Error      string    `json:"error,omitempty"`
	ErrorCode  string    `json:"errorCode,omitempty"`
}
type Manager struct {
	mu                    sync.Mutex
	status                Status
	current, root, source string
	network               func() Network
	cancel                context.CancelFunc
	closed                bool
	done                  chan struct{}
	file, digest          string
	asset                 Asset
	target, kind          string
	job                   *Job
}

func New(current, userDir string, network func() Network) *Manager {
	m := &Manager{current: current, root: filepath.Join(userDir, "updates"), network: network, status: Status{State: "idle"}}
	if raw, err := os.ReadFile(filepath.Join(m.root, "expected-version.txt")); err == nil {
		if strings.TrimSpace(string(raw)) != strings.TrimPrefix(current, "v") {
			m.status.Error = "The installed version did not change as expected. Please retry or use the website."
			m.status.State = "error"
		}
		_ = os.Remove(filepath.Join(m.root, "expected-version.txt"))
	}
	// Clean only updater-owned directories older than a day; never another active handoff.
	if entries, err := os.ReadDir(m.root); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || (!strings.HasPrefix(entry.Name(), "helper-") && !strings.HasPrefix(entry.Name(), "download-")) {
				continue
			}
			if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
				_ = os.RemoveAll(filepath.Join(m.root, entry.Name()))
			}
		}
	}
	// A helper leaves a concise failure for the next launch. Do not erase failed installers.
	if raw, err := os.ReadFile(filepath.Join(m.root, "last-error.txt")); err == nil {
		m.status.Error = string(raw)
		m.status.State = "error"
		_ = os.Remove(filepath.Join(m.root, "last-error.txt"))
	}
	return m
}
func (m *Manager) Status(locale string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	s.Website = metadata.InstallationURL(m.source, locale)
	if s.Manifest != nil {
		clone := *s.Manifest
		clone.Assets = append([]Asset(nil), s.Manifest.Assets...)
		s.Manifest = &clone
	}
	return s
}
func (m *Manager) Check(ctx context.Context) error {
	m.mu.Lock()
	if m.closed || m.status.State == "checking" || m.status.State == "downloading" || m.status.State == "ready" || m.status.State == "installing" {
		m.mu.Unlock()
		return nil
	}
	m.status.State = "checking"
	m.mu.Unlock()
	network := m.network()
	proxy := netproxy.Resolver(network.Port, network.Upstream, network.DownloadProxy, false)
	raw, source, err := metadata.FetchWithProxy(ctx, "/version.json", func(raw []byte) error { _, err := Decode(raw); return err }, proxy)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return context.Canceled
	}
	if err != nil {
		m.status.State = "error"
		m.status.Error = err.Error()
		m.status.ErrorCode = "check_network_failed"
		if errors.Is(err, ErrInvalidManifest) || errors.Is(err, metadata.ErrNotFound) {
			m.status.ErrorCode = "metadata_unavailable"
		}
		return err
	}
	manifest, err := Decode(raw)
	if err != nil {
		return err
	}
	m.source = source
	m.status.Manifest = &manifest
	m.status.Available = Newer(manifest.Version, m.current)
	m.status.State = "available"
	m.status.Error = ""
	m.status.ErrorCode = ""
	target, kind := installationTarget()
	m.target, m.kind = target, kind
	m.status.CanInstall = false
	for _, a := range manifest.Assets {
		if a.Platform == runtime.GOOS && (a.Arch == runtime.GOARCH || a.Arch == "universal") && a.Variant == kind {
			m.asset = a
			m.status.CanInstall = target != ""
			break
		}
	}
	return nil
}
func (m *Manager) Download(direct bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.status.State == "downloading" || m.status.State == "checking" || m.status.State == "installing" || m.status.State == "ready" {
		return errors.New("updater is busy")
	}
	if !m.status.Available || !m.status.CanInstall {
		return errors.New("this installation must be updated from the website")
	}
	if err := os.MkdirAll(m.root, 0700); err != nil {
		return err
	}
	folder, err := os.MkdirTemp(m.root, "download-")
	if err != nil {
		return err
	}
	// Remove only the previous download owned by this manager.
	if m.file != "" {
		_ = os.RemoveAll(filepath.Dir(m.file))
	}
	m.file = filepath.Join(folder, m.asset.Name)
	m.status.State = "downloading"
	m.status.Error = ""
	m.status.ErrorCode = ""
	m.status.Received = 0
	m.status.Total = m.asset.Size
	m.status.Speed = 0
	m.status.Slow = false
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	m.cancel = cancel
	m.done = make(chan struct{})
	asset, file, network := m.asset, m.file, m.network()
	go m.download(ctx, asset, file, network, direct, m.done)
	return nil
}
func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
}
func (m *Manager) download(ctx context.Context, asset Asset, file string, network Network, direct bool, done chan struct{}) {
	defer close(done)
	monitorCtx, stop := context.WithCancel(ctx)
	defer stop()
	monitorDone := make(chan struct{})
	go func() { defer close(monitorDone); m.monitor(monitorCtx) }()
	digest, err := m.fetch(ctx, asset, file, network, direct)
	stop()
	<-monitorDone
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.status.Speed = 0
	m.status.Slow = false
	if err != nil {
		_ = os.Remove(file + ".part")
		m.status.State = "error"
		m.status.Error = err.Error()
		if errors.Is(err, context.Canceled) {
			m.status.State = "available"
			m.status.Error = ""
		}
		return
	}
	m.digest = digest
	m.status.State = "ready"
	m.status.Received = asset.Size
}
func (m *Manager) monitor(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	started, lastProgress := time.Now(), time.Now()
	var previous int64
	var slowSince time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			m.mu.Lock()
			if m.status.State != "downloading" {
				m.mu.Unlock()
				return
			}
			delta := m.status.Received - previous
			previous = m.status.Received
			m.status.Speed = delta
			if delta > 0 {
				lastProgress = now
			}
			if now.Sub(started) > 10*time.Second && delta < 100*1024 {
				if slowSince.IsZero() {
					slowSince = now
				}
			} else {
				slowSince = time.Time{}
			}
			m.status.Slow = now.Sub(lastProgress) > 15*time.Second || (!slowSince.IsZero() && now.Sub(slowSince) > 20*time.Second)
			m.mu.Unlock()
		}
	}
}
func (m *Manager) fetch(ctx context.Context, asset Asset, file string, network Network, direct bool) (string, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = netproxy.Resolver(network.Port, network.Upstream, network.DownloadProxy, direct)
	transport.ResponseHeaderTimeout = 45 * time.Second
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: checkDownloadRedirect}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "res-downloader-updater")
	req.Header.Set("Accept-Encoding", "identity")
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update download: %s", response.Status)
	}
	if response.ContentLength >= 0 && response.ContentLength != asset.Size {
		return "", errors.New("update file size does not match")
	}
	output, err := os.OpenFile(file+".part", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	reader := io.LimitReader(response.Body, asset.Size+1)
	buffer := make([]byte, 128*1024)
	var received int64
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			if _, err = output.Write(buffer[:n]); err != nil {
				break
			}
			_, _ = hash.Write(buffer[:n])
			received += int64(n)
			m.mu.Lock()
			m.status.Received = received
			m.mu.Unlock()
		}
		if readErr != nil {
			if readErr != io.EOF {
				err = readErr
			}
			break
		}
	}
	closeErr := output.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if received != asset.Size {
		return "", errors.New("update download is incomplete")
	}
	if asset.SHA256 != "" && !strings.EqualFold(asset.SHA256, digest) {
		return "", errors.New("update SHA256 does not match; please download again")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return digest, os.Rename(file+".part", file)
}

// Prepare launches a copy of this executable in helper mode before requesting exit.
func (m *Manager) Prepare() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.status.State != "ready" {
		return errors.New("update is not ready")
	}
	job, err := prepareHelper(m.root, m.file, m.digest, m.target, m.kind, m.status.Manifest.Version)
	if err != nil {
		return err
	}
	m.job = job
	m.status.State = "installing"
	return nil
}

// Release is called only after application shutdown has released its resources.
func (m *Manager) Release(shutdownErr error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job == nil {
		return nil
	}
	if shutdownErr != nil {
		return os.WriteFile(filepath.Join(m.root, "last-error.txt"), []byte("Update postponed: application shutdown failed. Reopen the app and retry."), 0600)
	}
	return os.WriteFile(m.job.Gate, []byte("ready"), 0600)
}
