package httpapi

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

const defaultAPIRequestBodyLimit int64 = 64 * 1024 * 1024

var apiMethods = map[string]map[string]struct{}{
	"/api/preview":              {http.MethodGet: {}, http.MethodHead: {}},
	"/api/preview/hls":          {http.MethodGet: {}, http.MethodHead: {}},
	"/api/certificate/download": {http.MethodGet: {}},
}

func (h *Server) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.HandleAPI(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Server) HandleAPI(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api") {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" {
		if !trustedAPIOrigin(origin) {
			http.Error(w, "API request origin is not allowed", http.StatusForbidden)
			return true
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods(r.URL.Path), ", "))
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Range")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, Content-Type")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodOptions {
		if !knownAPIPath(r.URL.Path) {
			http.NotFound(w, r)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	if !knownAPIPath(r.URL.Path) {
		http.NotFound(w, r)
		return true
	}
	if !methodAllowed(r.URL.Path, r.Method) {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	if !h.authorizedAPIRequest(r) {
		http.Error(w, "API session is missing or invalid", http.StatusUnauthorized)
		return true
	}
	if r.Body != nil && r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, defaultAPIRequestBodyLimit)
	}
	switch r.URL.Path {
	case "/api/preview":
		h.preview(w, r)
	case "/api/preview/hls":
		h.previewHLSTarget(w, r)
	case "/api/proxy-open":
		h.openSystemProxy(w, r)
	case "/api/proxy-unset":
		h.unsetSystemProxy(w, r)
	case "/api/open-directory":
		h.openDirectoryDialog(w, r)
	case "/api/open-file":
		h.openFileDialog(w, r)
	case "/api/open-folder":
		h.openFolder(w, r)
	case "/api/is-proxy":
		h.isProxy(w, r)
	case "/api/app-info":
		h.appInfo(w, r)
	case "/api/set-config":
		h.setConfig(w, r)
	case "/api/get-config":
		h.getConfig(w, r)
	case "/api/media/status":
		h.mediaStatus(w, r)
	case "/api/certificate/status":
		h.certificateStatus(w, r)
	case "/api/certificate/install":
		h.installCurrentCertificate(w, r)
	case "/api/certificate/uninstall":
		h.uninstallCurrentCertificate(w, r)
	case "/api/certificate/cleanup":
		h.retryCertificateCleanup(w, r)
	case "/api/resources":
		h.listResources(w, r)
	case "/api/resources/filter":
		h.filterResources(w, r)
	case "/api/resources/clear":
		h.clearResources(w, r)
	case "/api/resources/delete":
		h.deleteResources(w, r)
	case "/api/resources/update":
		h.updateResource(w, r)
	case "/api/resources/action":
		h.resourceAction(w, r)
	case "/api/resources/import":
		h.importResources(w, r)
	case "/api/resources/export":
		h.exportResources(w, r)
	case "/api/download/create":
		h.createDownload(w, r)
	case "/api/download/tasks":
		h.downloadTasks(w, r)
	case "/api/download/retry":
		h.retryDownload(w, r)
	case "/api/download/pause":
		h.pauseDownloadTask(w, r)
	case "/api/download/resume":
		h.resumeDownloadTask(w, r)
	case "/api/download/cancel":
		h.cancelDownloadTask(w, r)
	case "/api/download/stop-recording":
		h.stopRecordingTask(w, r)
	case "/api/download/delete":
		h.deleteDownloadTask(w, r)
	case "/api/download/batch":
		h.batchDownloadTasks(w, r)
	case "/api/download/clear":
		h.clearDownloadTasks(w, r)
	case "/api/certificate/download":
		h.downloadCertificate(w, r)
	case "/api/plugins":
		h.pluginsAPI(w, r)
	case "/api/plugins/store":
		h.pluginStore(w, r)
	case "/api/plugins/reload":
		h.reloadPlugins(w, r)
	case "/api/plugins/install":
		h.installPlugin(w, r)
	case "/api/plugins/file/inspect":
		h.inspectPluginFile(w, r)
	case "/api/plugins/file/install":
		h.installPluginFile(w, r)
	case "/api/plugins/uninstall":
		h.uninstallPlugin(w, r)
	case "/api/plugins/rollback":
		h.rollbackPlugin(w, r)
	case "/api/plugins/enable":
		h.enablePlugin(w, r)
	case "/api/plugins/validate":
		h.validatePlugin(w, r)
	case "/api/plugins/settings":
		h.setPluginSettings(w, r)
	}
	return true
}

func knownAPIPath(path string) bool {
	switch path {
	case "/api/preview", "/api/preview/hls", "/api/proxy-open", "/api/proxy-unset",
		"/api/open-directory", "/api/open-file", "/api/open-folder", "/api/is-proxy",
		"/api/app-info", "/api/set-config", "/api/get-config", "/api/media/status",
		"/api/certificate/status", "/api/certificate/install", "/api/certificate/uninstall",
		"/api/certificate/cleanup", "/api/certificate/download", "/api/resources",
		"/api/resources/filter", "/api/resources/clear", "/api/resources/delete",
		"/api/resources/update", "/api/resources/action", "/api/resources/import", "/api/resources/export",
		"/api/download/create", "/api/download/tasks", "/api/download/retry",
		"/api/download/pause", "/api/download/resume", "/api/download/cancel",
		"/api/download/stop-recording", "/api/download/delete", "/api/download/batch",
		"/api/download/clear", "/api/plugins", "/api/plugins/store", "/api/plugins/reload",
		"/api/plugins/install", "/api/plugins/file/inspect", "/api/plugins/file/install",
		"/api/plugins/uninstall", "/api/plugins/rollback", "/api/plugins/enable",
		"/api/plugins/validate", "/api/plugins/settings":
		return true
	default:
		return false
	}
}

func allowedMethods(path string) []string {
	if methods, ok := apiMethods[path]; ok {
		result := make([]string, 0, len(methods)+1)
		if _, ok := methods[http.MethodGet]; ok {
			result = append(result, http.MethodGet)
		}
		if _, ok := methods[http.MethodHead]; ok {
			result = append(result, http.MethodHead)
		}
		return append(result, http.MethodOptions)
	}
	return []string{http.MethodPost, http.MethodOptions}
}

func methodAllowed(path, method string) bool {
	if methods, ok := apiMethods[path]; ok {
		_, allowed := methods[method]
		return allowed
	}
	return method == http.MethodPost
}

func trustedAPIOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme == "wails" {
		return host == "wails.localhost" || host == "wails"
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") &&
		(host == "wails.localhost" || host == "localhost" || host == "127.0.0.1" || host == "::1")
}

func (h *Server) authorizedAPIRequest(r *http.Request) bool {
	if r.URL.Path == "/api/certificate/download" || r.URL.Path == "/api/preview/hls" {
		return true
	}
	token := ""
	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(authorization, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	}
	if token == "" && r.URL.Path == "/api/preview" {
		token = strings.TrimSpace(r.URL.Query().Get("access_token"))
	}
	if token == "" || h.sessionToken == "" || len(token) != len(h.sessionToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(h.sessionToken)) == 1
}
