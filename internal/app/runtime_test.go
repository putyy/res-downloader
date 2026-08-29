package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elazarl/goproxy"
)

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
