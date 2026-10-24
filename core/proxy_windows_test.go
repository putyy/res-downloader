//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

// Exercise the Windows registry implementation without changing Internet Settings.
func newTestWindowsProxy(t *testing.T) (*windowsProxyManager, registry.Key) {
	t.Helper()
	path := fmt.Sprintf(`Software\res-downloader-proxy-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Error(err)
		}
	})
	return &windowsProxyManager{openKey: func() (windowsProxyKey, error) {
		return registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE|registry.SET_VALUE)
	}}, key
}

func proxyStateForTest(t *testing.T, key windowsProxyKey) windowsProxyState {
	t.Helper()
	state, err := readWindowsProxyState(key)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestWindowsProxyRestoresPreviousSettings(t *testing.T) {
	for _, previous := range []windowsProxyState{
		{enabled: 1, enabledExists: true, server: "127.0.0.1:7890", serverType: registry.SZ, serverExists: true},
		{enabledExists: true, server: "old-proxy:8080", serverType: registry.SZ, serverExists: true},
		{},
		{enabledExists: true, serverType: registry.SZ, serverExists: true},
		{enabled: 1, enabledExists: true, server: "%PROXY_HOST%:8080", serverType: registry.EXPAND_SZ, serverExists: true},
	} {
		t.Run(fmt.Sprintf("%+v", previous), func(t *testing.T) {
			manager, key := newTestWindowsProxy(t)
			if err := previous.restoreServer(key); err != nil {
				t.Fatal(err)
			}
			if err := previous.restoreEnabled(key); err != nil {
				t.Fatal(err)
			}
			if err := key.SetStringValue("AutoConfigURL", "https://example.invalid/proxy.pac"); err != nil {
				t.Fatal(err)
			}
			if err := key.SetStringValue("ProxyOverride", "<local>;*.internal"); err != nil {
				t.Fatal(err)
			}
			if err := manager.set("127.0.0.1:8899"); err != nil {
				t.Fatal(err)
			}
			want := windowsProxyState{enabled: 1, enabledExists: true, server: "127.0.0.1:8899", serverType: registry.SZ, serverExists: true}
			if got := proxyStateForTest(t, key); got != want {
				t.Fatalf("acquired proxy = %+v, want %+v", got, want)
			}
			// A duplicate start must retain both the original snapshot and owned endpoint.
			if err := manager.set("127.0.0.1:9000"); err != nil {
				t.Fatal(err)
			}
			if got := proxyStateForTest(t, key); got != want {
				t.Fatalf("duplicate start changed proxy: %+v", got)
			}
			for i := 0; i < 2; i++ {
				if err := manager.unset(); err != nil {
					t.Fatal(err)
				}
				if got := proxyStateForTest(t, key); got != previous {
					t.Fatalf("restored proxy = %+v, want %+v", got, previous)
				}
			}
			for name, want := range map[string]string{"AutoConfigURL": "https://example.invalid/proxy.pac", "ProxyOverride": "<local>;*.internal"} {
				if got, _, err := key.GetStringValue(name); err != nil || got != want {
					t.Fatalf("%s changed: %q, %v", name, got, err)
				}
			}
		})
	}
}

func TestWindowsProxyPreservesExternalChanges(t *testing.T) {
	for name, change := range map[string]func(registry.Key) error{
		"another proxy":   func(key registry.Key) error { return key.SetStringValue("ProxyServer", "127.0.0.1:7891") },
		"disabled proxy":  func(key registry.Key) error { return key.SetDWordValue("ProxyEnable", 0) },
		"deleted address": func(key registry.Key) error { return key.DeleteValue("ProxyServer") },
		"deleted switch":  func(key registry.Key) error { return key.DeleteValue("ProxyEnable") },
	} {
		t.Run(name, func(t *testing.T) {
			manager, key := newTestWindowsProxy(t)
			if err := key.SetStringValue("ProxyServer", "original:8080"); err != nil {
				t.Fatal(err)
			}
			if err := manager.set("127.0.0.1:8899"); err != nil {
				t.Fatal(err)
			}
			if err := change(key); err != nil {
				t.Fatal(err)
			}
			external := proxyStateForTest(t, key)
			if err := manager.unset(); err != nil {
				t.Fatal(err)
			}
			if got := proxyStateForTest(t, key); got != external {
				t.Fatalf("overwrote external settings: %+v, want %+v", got, external)
			}
			// A later capture must take a fresh snapshot after ownership was lost.
			if err := manager.set("127.0.0.1:9000"); err != nil {
				t.Fatal(err)
			}
			if err := manager.unset(); err != nil {
				t.Fatal(err)
			}
			if got := proxyStateForTest(t, key); got != external {
				t.Fatalf("reused stale snapshot: %+v, want %+v", got, external)
			}
		})
	}
}

type failingWindowsProxyKey struct {
	windowsProxyKey
	failEnable bool
}

func (key *failingWindowsProxyKey) SetDWordValue(name string, value uint32) error {
	if name == "ProxyEnable" && key.failEnable {
		key.failEnable = false
		return registry.ErrUnexpectedType
	}
	return key.windowsProxyKey.SetDWordValue(name, value)
}

func TestWindowsProxyRollsBackFailedStart(t *testing.T) {
	manager, key := newTestWindowsProxy(t)
	before := proxyStateForTest(t, key)
	openKey := manager.openKey
	manager.openKey = func() (windowsProxyKey, error) {
		key, err := openKey()
		return &failingWindowsProxyKey{windowsProxyKey: key, failEnable: true}, err
	}
	if err := manager.set("127.0.0.1:8899"); err == nil {
		t.Fatal("start unexpectedly succeeded")
	}
	if got := proxyStateForTest(t, key); got != before {
		t.Fatalf("failed start left changed settings: %+v", got)
	}
	if manager.previous != nil {
		t.Fatal("failed start retained ownership")
	}
}

func TestWindowsProxyRetriesPartialRestore(t *testing.T) {
	manager, key := newTestWindowsProxy(t)
	if err := key.SetDWordValue("ProxyEnable", 0); err != nil {
		t.Fatal(err)
	}
	before := proxyStateForTest(t, key)
	if err := manager.set("127.0.0.1:8899"); err != nil {
		t.Fatal(err)
	}
	openKey := manager.openKey
	manager.openKey = func() (windowsProxyKey, error) {
		key, err := openKey()
		return &failingWindowsProxyKey{windowsProxyKey: key, failEnable: true}, err
	}
	if err := manager.unset(); err == nil {
		t.Fatal("restore unexpectedly succeeded")
	}
	manager.openKey = openKey
	if err := manager.unset(); err != nil {
		t.Fatal(err)
	}
	if got := proxyStateForTest(t, key); got != before {
		t.Fatalf("retry did not restore settings: %+v, want %+v", got, before)
	}
}

func TestWindowsProxyRejectsUnreadableSnapshot(t *testing.T) {
	manager, key := newTestWindowsProxy(t)
	if err := key.SetStringValue("ProxyServer", "original:8080"); err != nil {
		t.Fatal(err)
	}
	if err := key.SetStringValue("ProxyEnable", "invalid"); err != nil {
		t.Fatal(err)
	}
	if err := manager.set("127.0.0.1:8899"); !errors.Is(err, registry.ErrUnexpectedType) {
		t.Fatalf("unexpected snapshot error: %v", err)
	}
	if server, _, err := key.GetStringValue("ProxyServer"); err != nil || server != "original:8080" {
		t.Fatalf("changed proxy despite unreadable snapshot: %q, %v", server, err)
	}
}
