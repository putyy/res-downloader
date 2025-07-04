package core

import (
	"net/http"
	"strings"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if HandleApi(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func HandleApi(w http.ResponseWriter, r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api") {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return true
		}
		switch r.URL.Path {
		case "/api/install":
			httpServerOnce.install(w, r)
		case "/api/set-system-password":
			httpServerOnce.setSystemPassword(w, r)
		case "/api/preview":
			httpServerOnce.preview(w, r)
		case "/api/proxy-open":
			httpServerOnce.openSystemProxy(w, r)
		case "/api/proxy-unset":
			httpServerOnce.unsetSystemProxy(w, r)
		case "/api/open-directory":
			httpServerOnce.openDirectoryDialog(w, r)
		case "/api/open-file":
			httpServerOnce.openFileDialog(w, r)
		case "/api/open-folder":
			httpServerOnce.openFolder(w, r)
		case "/api/is-proxy":
			httpServerOnce.isProxy(w, r)
		case "/api/app-info":
			httpServerOnce.appInfo(w, r)
		case "/api/set-config":
			httpServerOnce.setConfig(w, r)
		case "/api/get-config":
			httpServerOnce.getConfig(w, r)
		case "/api/set-type":
			httpServerOnce.setType(w, r)
		case "/api/clear":
			httpServerOnce.clear(w, r)
		case "/api/delete":
			httpServerOnce.delete(w, r)
		case "/api/download":
			httpServerOnce.download(w, r)
		case "/api/wx-file-decode":
			httpServerOnce.wxFileDecode(w, r)
		case "/api/batch-export":
			httpServerOnce.batchExport(w, r)
		case "/api/cert":
			httpServerOnce.downCert(w, r)
		}
		return true
	}
	return false
}
