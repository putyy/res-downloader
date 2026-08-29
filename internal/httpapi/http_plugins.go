package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (h *Server) pluginsAPI(w http.ResponseWriter, _ *http.Request) {
	h.success(w, respData{
		"plugins":  h.plugins.Statuses(),
		"settings": h.plugins.Settings(),
	})
}

func (h *Server) pluginStore(w http.ResponseWriter, r *http.Request) {
	if !trustedPluginManagementRequest(r) {
		h.error(w, "plugin management request origin is not allowed")
		return
	}
	index, stale, warning, err := h.plugins.PluginStore(r.Context())
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"index":   index,
		"stale":   stale,
		"warning": warning,
	})
}

func (h *Server) installPlugin(w http.ResponseWriter, r *http.Request) {
	if !trustedPluginManagementRequest(r) {
		h.error(w, "plugin management request origin is not allowed")
		return
	}
	var data struct {
		ID                 string `json:"id"`
		Version            string `json:"version"`
		ApprovePermissions bool   `json:"approvePermissions"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	manifest, source, err := h.plugins.InstallFromStore(r.Context(), strings.TrimSpace(data.ID), strings.TrimSpace(data.Version), data.ApprovePermissions)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{"manifest": manifest, "plugins": h.plugins.Statuses(), "source": source})
}

func (h *Server) inspectPluginFile(w http.ResponseWriter, r *http.Request) {
	if !trustedPluginManagementRequest(r) {
		h.error(w, "plugin management request origin is not allowed")
		return
	}
	fileName, err := runtime.OpenFileDialog(h.host.context(), runtime.OpenDialogOptions{
		Title:   "Select a plugin ZIP",
		Filters: []runtime.FileFilter{{DisplayName: "Plugin ZIP (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	if fileName == "" {
		h.success(w, respData{"cancelled": true})
		return
	}
	info, err := os.Stat(fileName)
	if err != nil || !info.Mode().IsRegular() {
		h.error(w, "selected plugin ZIP is not a regular file")
		return
	}
	if info.Size() <= 0 || info.Size() > plugin.MaxPluginArchiveSize {
		h.error(w, fmt.Sprintf("plugin package must be between 1 and %d bytes", plugin.MaxPluginArchiveSize))
		return
	}
	archive, err := os.ReadFile(fileName)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	manifest, digest, err := plugin.InspectLocalPluginArchiveDetails(archive)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	token, err := h.rememberPluginArchive(archive, manifest, digest)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	match := h.plugins.LocalArchiveStoreMatch(manifest)
	var installed interface{}
	for _, status := range h.plugins.Statuses() {
		if status.Manifest.ID == manifest.ID {
			installed = respData{
				"version": status.Manifest.Version, "builtin": status.Builtin, "bundled": status.Bundled,
			}
			break
		}
	}
	h.success(w, respData{
		"token": token, "manifest": manifest, "contentSha256": digest,
		"storeMatch": match, "installed": installed,
	})
}

func (h *Server) installPluginFile(w http.ResponseWriter, r *http.Request) {
	if !trustedPluginManagementRequest(r) {
		h.error(w, "plugin management request origin is not allowed")
		return
	}
	var data struct {
		Token              string `json:"token"`
		Replace            bool   `json:"replace"`
		ApprovePermissions bool   `json:"approvePermissions"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	pending, err := h.takePluginArchive(data.Token)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	manifest, err := h.plugins.InstallLocalArchiveApproved(
		pending.data, data.Replace, pending.manifest.ID, pending.manifest.Version, pending.digest, data.ApprovePermissions,
	)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{"manifest": manifest, "plugins": h.plugins.Statuses()})
}

func (h *Server) rollbackPlugin(w http.ResponseWriter, r *http.Request) {
	if !trustedPluginManagementRequest(r) {
		h.error(w, "plugin management request origin is not allowed")
		return
	}
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	manifest, err := h.plugins.Rollback(data.ID)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{"manifest": manifest, "plugins": h.plugins.Statuses()})
}

func (h *Server) rememberPluginArchive(data []byte, manifest shared.PluginManifest, digest string) (string, error) {
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	token := hex.EncodeToString(random)
	h.pluginArchiveMu.Lock()
	defer h.pluginArchiveMu.Unlock()
	now := time.Now()
	for key, archive := range h.pluginArchives {
		if now.After(archive.expiresAt) {
			delete(h.pluginArchives, key)
		}
	}
	if len(h.pluginArchives) >= 5 {
		return "", errors.New("too many plugin packages are awaiting confirmation")
	}
	h.pluginArchives[token] = pendingPluginArchive{
		data: append([]byte(nil), data...), manifest: manifest, digest: digest, expiresAt: now.Add(10 * time.Minute),
	}
	time.AfterFunc(10*time.Minute, func() {
		h.pluginArchiveMu.Lock()
		defer h.pluginArchiveMu.Unlock()
		if archive, exists := h.pluginArchives[token]; exists && !time.Now().Before(archive.expiresAt) {
			delete(h.pluginArchives, token)
		}
	})
	return token, nil
}

func (h *Server) takePluginArchive(token string) (pendingPluginArchive, error) {
	if len(token) != 48 {
		return pendingPluginArchive{}, errors.New("invalid or expired plugin package token")
	}
	h.pluginArchiveMu.Lock()
	defer h.pluginArchiveMu.Unlock()
	pending, exists := h.pluginArchives[token]
	delete(h.pluginArchives, token)
	if !exists || time.Now().After(pending.expiresAt) {
		return pendingPluginArchive{}, errors.New("invalid or expired plugin package token")
	}
	return pending, nil
}

func (h *Server) uninstallPlugin(w http.ResponseWriter, r *http.Request) {
	if !trustedPluginManagementRequest(r) {
		h.error(w, "plugin management request origin is not allowed")
		return
	}
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if err := h.plugins.Uninstall(data.ID); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, h.plugins.Statuses())
}

func trustedPluginManagementRequest(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme == "wails" && (host == "wails.localhost" || host == "wails") {
		return true
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") &&
		(host == "wails.localhost" || host == "localhost" || host == "127.0.0.1" || host == "::1")
}

func (h *Server) reloadPlugins(w http.ResponseWriter, _ *http.Request) {
	if err := h.plugins.Reload(); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, h.plugins.Statuses())
}

func (h *Server) enablePlugin(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if err := h.plugins.SetEnabled(data.ID, data.Enabled); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, h.plugins.Statuses())
}

func (h *Server) validatePlugin(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if err := h.plugins.Validate(data.ID); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w)
}

func (h *Server) setPluginSettings(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID       string                 `json:"id"`
		Settings map[string]interface{} `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if err := h.plugins.SetSettings(data.ID, data.Settings); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w)
}
