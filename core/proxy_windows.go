//go:build windows

package core

import (
	"errors"
	"sync"

	"golang.org/x/sys/windows/registry"
)

type windowsProxyKey interface {
	GetIntegerValue(string) (uint64, uint32, error)
	GetStringValue(string) (string, uint32, error)
	SetDWordValue(string, uint32) error
	SetStringValue(string, string) error
	SetExpandStringValue(string, string) error
	DeleteValue(string) error
	Close() error
}

type windowsProxyState struct {
	enabled       uint32
	enabledExists bool
	server        string
	serverType    uint32
	serverExists  bool
}

type windowsProxyManager struct {
	mu       sync.Mutex
	openKey  func() (windowsProxyKey, error)
	previous *windowsProxyState
	expected windowsProxyState
}

var windowsSystemProxy = windowsProxyManager{
	openKey: func() (windowsProxyKey, error) {
		return registry.OpenKey(registry.CURRENT_USER,
			`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
			registry.QUERY_VALUE|registry.SET_VALUE)
	},
}

func readWindowsProxyState(key windowsProxyKey) (windowsProxyState, error) {
	var state windowsProxyState
	enabled, valueType, err := key.GetIntegerValue("ProxyEnable")
	if err == nil {
		if valueType != registry.DWORD {
			return state, registry.ErrUnexpectedType
		}
		state.enabled, state.enabledExists = uint32(enabled), true
	} else if !errors.Is(err, registry.ErrNotExist) {
		return state, err
	}
	server, valueType, err := key.GetStringValue("ProxyServer")
	if err == nil {
		state.server, state.serverType, state.serverExists = server, valueType, true
	} else if !errors.Is(err, registry.ErrNotExist) {
		return state, err
	}
	return state, nil
}

func (state windowsProxyState) restoreServer(key windowsProxyKey) error {
	if !state.serverExists {
		err := key.DeleteValue("ProxyServer")
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	if state.serverType == registry.EXPAND_SZ {
		return key.SetExpandStringValue("ProxyServer", state.server)
	}
	return key.SetStringValue("ProxyServer", state.server)
}

func (state windowsProxyState) restoreEnabled(key windowsProxyKey) error {
	if state.enabledExists {
		return key.SetDWordValue("ProxyEnable", state.enabled)
	}
	err := key.DeleteValue("ProxyEnable")
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

func (m *windowsProxyManager) set(server string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Repeated requests must not replace the original snapshot with our proxy.
	if m.previous != nil {
		return nil
	}
	key, err := m.openKey()
	if err != nil {
		return err
	}
	defer key.Close()
	previous, err := readWindowsProxyState(key)
	if err != nil {
		return err
	}
	if err = key.SetStringValue("ProxyServer", server); err != nil {
		return err
	}
	if err = key.SetDWordValue("ProxyEnable", 1); err != nil {
		return errors.Join(err, previous.restoreServer(key))
	}
	m.previous = &previous
	m.expected = windowsProxyState{
		enabled: 1, enabledExists: true,
		server: server, serverType: registry.SZ, serverExists: true,
	}
	return nil
}

func (m *windowsProxyManager) unset() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.previous == nil {
		return nil
	}
	key, err := m.openKey()
	if err != nil {
		return err
	}
	defer key.Close()
	current, err := readWindowsProxyState(key)
	if err != nil {
		return err
	}
	// Another application (or the user) may have taken over since we started.
	// Only restore settings while they still match what we last wrote.
	if current != m.expected {
		m.previous = nil
		return nil
	}
	if err = m.previous.restoreServer(key); err != nil {
		return err
	}
	// Remember a partial restore so a failed write can be retried safely.
	m.expected.server = m.previous.server
	m.expected.serverType = m.previous.serverType
	m.expected.serverExists = m.previous.serverExists
	if err = m.previous.restoreEnabled(key); err != nil {
		return err
	}
	m.previous = nil
	return nil
}
