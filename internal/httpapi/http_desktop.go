package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"res-downloader/internal/config"
	shared "res-downloader/internal/model"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (h *Server) downloadCertificate(w http.ResponseWriter, _ *http.Request) {
	var certificate []byte
	if h.host.PublicCertificate != nil {
		certificate = h.host.PublicCertificate()
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-data")
	w.Header().Set("Content-Disposition", "attachment;filename=res-downloader-public.crt")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(certificate)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewReader(certificate))
}

func (h *Server) openDirectoryDialog(w http.ResponseWriter, _ *http.Request) {
	folder, err := runtime.OpenDirectoryDialog(h.host.context(), runtime.OpenDialogOptions{
		DefaultDirectory: "",
		Title:            "Select a folder",
	})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{"folder": folder})
}

func (h *Server) openFileDialog(w http.ResponseWriter, _ *http.Request) {
	filePath, err := runtime.OpenFileDialog(h.host.context(), runtime.OpenDialogOptions{
		Filters: []runtime.FileFilter{{DisplayName: "Videos (*.mov;*.mp4)", Pattern: "*.mp4"}},
		Title:   "Select a file",
	})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{"file": filePath})
}

func (h *Server) openFolder(w http.ResponseWriter, r *http.Request) {
	var data struct {
		FilePath string `json:"filePath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err == nil && data.FilePath == "" {
		return
	}
	if err := shared.OpenFolder(data.FilePath); err != nil {
		h.logger.Err(err)
		h.error(w, err.Error())
		return
	}
	h.success(w)
}

func (h *Server) openSystemProxy(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Password string `json:"password"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&data)
	}
	if h.host.OpenSystemProxy == nil {
		h.error(w, "system proxy setup is unavailable")
		return
	}
	if err := h.host.OpenSystemProxy(data.Password); err != nil {
		h.error(w, err.Error(), respData{"value": h.host.proxyEnabled()})
		return
	}
	h.success(w, respData{"value": h.host.proxyEnabled()})
}

func (h *Server) unsetSystemProxy(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Password string `json:"password"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&data)
	}
	if h.host.UnsetSystemProxy == nil {
		h.error(w, "system proxy setup is unavailable")
		return
	}
	if err := h.host.UnsetSystemProxy(data.Password); err != nil {
		h.error(w, err.Error(), respData{"value": h.host.proxyEnabled()})
		return
	}
	h.success(w, respData{"value": h.host.proxyEnabled()})
}

func (h *Server) isProxy(w http.ResponseWriter, _ *http.Request) {
	h.success(w, respData{"value": h.host.proxyEnabled()})
}

func (h *Server) appInfo(w http.ResponseWriter, _ *http.Request) {
	h.success(w, h.host.AppInfo)
}

func (h *Server) getConfig(w http.ResponseWriter, _ *http.Request) {
	h.success(w, h.config.Snapshot())
}

func (h *Server) mediaStatus(w http.ResponseWriter, _ *http.Request) {
	h.success(w, h.media.Detect())
}

func (h *Server) certificateStatus(w http.ResponseWriter, _ *http.Request) {
	if h.host.CertificateStatus == nil {
		h.error(w, "certificate status is unavailable")
		return
	}
	h.success(w, h.host.CertificateStatus())
}

func (h *Server) installCurrentCertificate(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Password string `json:"password"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&data)
	}
	if h.host.InstallCertificateWithPass == nil {
		h.error(w, "certificate installation is unavailable")
		return
	}
	out, err := h.host.InstallCertificateWithPass(data.Password)
	if err != nil {
		message := err.Error()
		if detail := strings.TrimSpace(out); detail != "" {
			message += "\n" + detail
		}
		h.error(w, message)
		return
	}
	h.success(w, h.host.CertificateStatus())
}

func (h *Server) uninstallCurrentCertificate(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Password string `json:"password"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&data)
	}
	if h.host.UninstallCertificate == nil {
		h.error(w, "certificate removal is unavailable")
		return
	}
	out, err := h.host.UninstallCertificate(data.Password)
	if err != nil {
		message := err.Error()
		if detail := strings.TrimSpace(out); detail != "" {
			message += "\n" + detail
		}
		h.error(w, message)
		return
	}
	h.success(w, h.host.CertificateStatus())
}

func (h *Server) retryCertificateCleanup(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Password string `json:"password"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&data)
	}
	if h.host.RetryCertificateCleanup == nil {
		h.error(w, "certificate cleanup is unavailable")
		return
	}
	h.success(w, h.host.RetryCertificateCleanup(data.Password))
}

func (h *Server) setConfig(w http.ResponseWriter, r *http.Request) {
	var data config.Config
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if err := h.config.Apply(data); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w)
}
