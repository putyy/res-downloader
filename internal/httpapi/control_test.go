package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestControlTokenIsScopedToAutomation(t *testing.T) {
	h := &Server{sessionToken: "desktop-token"}
	handler := h.ControlHandler("control-token")
	for _, tc := range []struct {
		path, method, token string
		want                int
	}{
		{"/api/download/tasks", http.MethodPost, "", http.StatusUnauthorized},
		{"/api/download/tasks", http.MethodPost, "desktop-token", http.StatusUnauthorized},
		{"/api/set-config", http.MethodPost, "control-token", http.StatusNotFound},
		{"/api/certificate/install", http.MethodPost, "control-token", http.StatusNotFound},
		{"/api/resources", http.MethodGet, "control-token", http.StatusMethodNotAllowed},
		{"/api/download/tasks", http.MethodPost, "control-token", http.StatusOK},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/api/download/tasks", nil)
	r.Header.Set("Authorization", "Bearer control-token")
	if h.authorizedAPIRequest(r) {
		t.Fatal("control token authorized the desktop API")
	}
}
