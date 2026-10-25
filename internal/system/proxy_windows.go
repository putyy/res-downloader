//go:build windows

package system

import (
	"errors"
	"golang.org/x/sys/windows/registry"
)

type registryProxyValue struct {
	kind   uint32
	text   string
	number uint32
}
type registryProxy [2]registryProxyValue
type windowsProxyStore struct{}

var proxyValueNames = [2]string{"ProxyServer", "ProxyEnable"}

func proxyRegistryKey() (registry.Key, error) {
	return registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE|registry.SET_VALUE)
}

func (windowsProxyStore) read(string) (any, error) {
	key, err := proxyRegistryKey()
	if err != nil {
		return nil, err
	}
	defer key.Close()
	var value registryProxy
	text, kind, err := key.GetStringValue("ProxyServer")
	if err == nil {
		value[0] = registryProxyValue{kind: kind, text: text}
	} else if !errors.Is(err, registry.ErrNotExist) {
		return nil, err
	}
	number, kind, err := key.GetIntegerValue("ProxyEnable")
	if err == nil {
		if kind != registry.DWORD {
			return nil, registry.ErrUnexpectedType
		}
		value[1] = registryProxyValue{kind: kind, number: uint32(number)}
	} else if !errors.Is(err, registry.ErrNotExist) {
		return nil, err
	}
	return value, nil
}

func (windowsProxyStore) write(_ string, current, target any) (any, error) {
	value, desired := current.(registryProxy), target.(registryProxy)
	key, err := proxyRegistryKey()
	if err != nil {
		return value, err
	}
	defer key.Close()
	for i, name := range proxyValueNames {
		v := desired[i]
		if value[i] == v {
			continue
		}
		switch v.kind {
		case 0:
			err = key.DeleteValue(name)
		case registry.SZ:
			err = key.SetStringValue(name, v.text)
		case registry.EXPAND_SZ:
			err = key.SetExpandStringValue(name, v.text)
		case registry.DWORD:
			err = key.SetDWordValue(name, v.number)
		default:
			err = registry.ErrUnexpectedType
		}
		if err != nil && !(v.kind == 0 && errors.Is(err, registry.ErrNotExist)) {
			return value, err
		}
		value[i] = v
	}
	return value, nil
}
