package control

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSessionRejectsNonLoopbackEndpoints(t *testing.T) {
	path := SessionPath(t.TempDir())
	for _, endpoint := range []string{"http://example.com:1234", "http://localhost:1234", "https://127.0.0.1:1234", "http://127.0.0.1:1234/private", "http://user@127.0.0.1:1234", "http://127.0.0.1:0"} {
		if err := writeSession(path, Session{Version: 1, URL: endpoint, Token: strings.Repeat("a", 64)}); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadSession(path); err == nil {
			t.Fatalf("accepted endpoint %q", endpoint)
		}
	}
}

func TestServerDiscoveryOriginAndCleanup(t *testing.T) {
	dir := t.TempDir()
	start := func() *Server {
		s, err := Start(dir, func(string) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close(context.Background()) })
		return s
	}
	first := start()
	session, err := ReadSession(SessionPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(SessionPath(dir))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatal("session credential file is not private")
		}
	}
	request, _ := http.NewRequest(http.MethodPost, session.URL+"/api/resources", nil)
	request.Header.Set("Origin", "http://localhost")
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatal("browser origin was accepted")
	}
	second := start()
	if err := first.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, err := ReadSession(SessionPath(dir))
	if err != nil || current.Token != second.session.Token {
		t.Fatal("old server removed the newer discovery file")
	}
	if err := second.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SessionPath(dir)); !os.IsNotExist(err) {
		t.Fatal("session file remains after shutdown")
	}
}

func TestInvalidCredentialIsNotEchoed(t *testing.T) {
	path := SessionPath(t.TempDir())
	secret := strings.Repeat("Z", 64)
	if err := writeSession(path, Session{Version: 1, URL: "http://127.0.0.1:1234", Token: secret}); err != nil {
		t.Fatal(err)
	}
	_, err := ReadSession(path)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatal("invalid credential accepted or exposed")
	}
	// Verify the session format remains JSON, without printing its contents.
	raw, err := os.ReadFile(path)
	if err != nil || !json.Valid(raw) {
		t.Fatal("invalid session encoding")
	}
}
