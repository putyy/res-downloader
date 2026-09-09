package httpapi

import (
	"crypto/subtle"
	"net/http"
)

// ControlHandler exposes only operations that are usable without dialogs. Its
// token is distinct from the Wails session token and cannot authorize requests
// on the capture gateway or desktop-only routes.
func (h *Server) ControlHandler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "local session is missing or invalid", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/resources", "/api/download/create", "/api/download/tasks",
			"/api/download/pause", "/api/download/resume", "/api/download/cancel", "/api/download/retry":
		default:
			http.NotFound(w, r)
			return
		}
		request := r.Clone(r.Context())
		request.Header.Set("Authorization", "Bearer "+h.sessionToken)
		h.HandleAPI(w, request)
	})
}
