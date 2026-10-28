package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"res-downloader/internal/httpapi"

	"github.com/elazarl/goproxy"
)

func TestPrepareRuntimeLeavesBusinessStateUntouched(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"WindowWidth":1100,"WindowHeight":700}`), 0600); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, "install.lock")
	if err := os.WriteFile(legacy, []byte("legacy"), 0600); err != nil {
		t.Fatal(err)
	}
	r := prepareRuntime(&App{UserDir: dir, AppName: "res-downloader", Version: "4.0.0"})
	t.Cleanup(func() { _ = r.App.shutdown() })
	if width, height := r.Config.WindowSize(); width != 1100 || height != 700 {
		t.Fatalf("saved window size not available before startup: %dx%d", width, height)
	}
	for _, name := range []string{"resources.db", "tasks.db", "mitm-ca.key", "capture-cache", "plugins", "updates"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("business state %s accessed before single-instance check: %v", name, err)
		}
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy cleanup ran before single-instance check: %v", err)
	}
	if r.HTTP != nil || r.Downloads != nil || r.Resources != nil || r.Plugins != nil || r.Updates != nil {
		t.Fatal("business services constructed before startup")
	}
}

func TestRuntimeCloseBeforeStartup(t *testing.T) {
	r := &Runtime{App: &App{}, startupDone: make(chan struct{})}
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err == nil {
		t.Fatal("startup proceeded after the window was closed")
	}
	select {
	case <-r.startupDone:
	default:
		t.Fatal("startup waiters were not released")
	}
	if r.HTTP != nil || r.Resources != nil || r.Downloads != nil {
		t.Fatal("closed runtime created business resources")
	}
}

func TestRuntimeMiddlewareServesAssetsBeforeStartup(t *testing.T) {
	r := &Runtime{startupDone: make(chan struct{})}
	handler := r.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("frontend asset blocked by backend startup: %d", response.Code)
	}
}

func TestRuntimeMiddlewareWaitsAndKeepsAuthentication(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "ready"
		if failed {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) {
			r := &Runtime{startupDone: make(chan struct{})}
			handler := r.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("API request fell through to frontend assets")
			}))
			response := httptest.NewRecorder()
			finished := make(chan struct{})
			go func() {
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/app-info", nil))
				close(finished)
			}()
			select {
			case <-finished:
				t.Fatal("API request did not wait for startup")
			case <-time.After(20 * time.Millisecond):
			}
			if failed {
				r.startupErr = errors.New("initialisation failed")
			} else {
				r.HTTP = httpapi.New(httpapi.Host{}, "session", nil, nil, nil, nil, nil, nil, nil)
			}
			close(r.startupDone)
			<-finished
			want := http.StatusUnauthorized
			if failed {
				want = http.StatusServiceUnavailable
			}
			if response.Code != want {
				t.Fatalf("API status = %d, want %d", response.Code, want)
			}
		})
	}
}

func TestRuntimeCloseCancelsStartupBeforeWaitingForResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &Runtime{App: &App{}, startupCancel: cancel}
	// Model an initialization stage holding the lifecycle lock. Cancellation
	// must reach it before Close can acquire that lock to release resources.
	r.lifecycleMu.Lock()
	closed := make(chan error, 1)
	go func() { closed <- r.Close(context.Background()) }()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		r.lifecycleMu.Unlock()
		t.Fatal("cleanup waited for initialization without cancelling it")
	}
	r.lifecycleMu.Unlock()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}

func TestCancelledInitialisationLeavesBusinessStateUntouched(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &Runtime{App: &App{UserDir: t.TempDir()}}
	if err := r.initialise(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled startup returned %v", err)
	}
	entries, err := os.ReadDir(r.App.UserDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancelled startup created application state: %v, %v", entries, err)
	}
}

func TestRemoveLegacyLocalFiles(t *testing.T) {
	userDir := t.TempDir()
	legacyFiles := []string{"pass.cache", "install.lock", "cert.crt"}
	for _, name := range legacyFiles {
		if err := os.WriteFile(filepath.Join(userDir, name), []byte("legacy data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := removeLegacyLocalFiles(userDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range legacyFiles {
		if _, err := os.Stat(filepath.Join(userDir, name)); !os.IsNotExist(err) {
			t.Fatalf("legacy file %s still exists: %v", name, err)
		}
	}
	if err := removeLegacyLocalFiles(userDir); err != nil {
		t.Fatalf("cleanup should be idempotent: %v", err)
	}
}

func TestHTTPServerLifecycleUsesInstanceDependencies(t *testing.T) {
	config := &Config{Host: "127.0.0.1", Port: "0"}
	logger := NewLogger(false, "")
	proxy := &Proxy{Proxy: goproxy.NewProxyHttpServer()}
	server := NewHTTPServer(
		&App{}, "test-session", config, proxy, &Resource{}, &PluginManager{}, &DownloadScheduler{},
		NewMediaEngine(config), &SystemSetup{}, logger,
	)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	if !server.Active() {
		t.Fatal("HTTP server did not retain its lifecycle handles")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestConfigApplyUsesRuntimeHook(t *testing.T) {
	config := newConfig(&App{UserDir: t.TempDir()}, NewLogger(false, ""))
	config.FilenameTemplate = "{{title}}.{{ext}}"
	config.FilenameConflict = "rename"
	config.InterceptionPolicies = []InterceptionPolicy{{ID: "default", Name: "Default", Enabled: true, Domains: []string{"*"}, Action: InterceptionActionMITM}}
	called := false
	config.SetApplyHook(func(previous, current Config) error {
		called = true
		if previous.UpstreamProxy == current.UpstreamProxy || len(current.InterceptionPolicies) != 1 {
			t.Fatalf("unexpected hook values: previous=%q current=%q policies=%d", previous.UpstreamProxy, current.UpstreamProxy, len(current.InterceptionPolicies))
		}
		return nil
	})
	next := config.Snapshot()
	next.UpstreamProxy = "http://127.0.0.1:7890"
	if err := config.Apply(next); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("runtime config hook was not called")
	}
}
