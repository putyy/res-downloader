package system

import (
	"errors"
	"reflect"
	"sync"
)

// Stores are operation-scoped; only values survive WithPassword copies.
type proxyStore interface {
	read(string) (any, error)
	// Return the last successfully written state even when a later write fails.
	write(string, any, any) (any, error)
}

type proxyChange struct {
	key         string
	before, own any
}

type proxyState struct {
	mu      sync.Mutex
	changes []proxyChange
	active  bool
}

func newProxyState() *proxyState { return &proxyState{} }

func (p *proxyState) set(store proxyStore, targets []proxyChange) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active {
		return nil
	}
	if err := p.restore(store); err != nil {
		return err
	}
	changes := make([]proxyChange, len(targets))
	for i, target := range targets {
		value, err := store.read(target.key)
		if err != nil {
			return err
		}
		changes[i] = proxyChange{key: target.key, before: value, own: value}
	}
	p.changes = changes
	for i, target := range targets {
		change := &p.changes[i]
		value, err := store.write(change.key, change.own, target.own)
		change.own = value
		if err != nil {
			return errors.Join(err, p.restore(store))
		}
	}
	p.active = true
	return nil
}

func (p *proxyState) unset(store proxyStore) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.restore(store)
}

func (p *proxyState) restore(store proxyStore) error {
	var pending []proxyChange
	var errs []error
	for i := len(p.changes) - 1; i >= 0; i-- {
		change := p.changes[i]
		value, err := store.read(change.key)
		if err == nil && !reflect.DeepEqual(value, change.own) {
			continue
		}
		if err == nil {
			change.own, err = store.write(change.key, change.own, change.before)
		}
		if err != nil {
			pending = append(pending, change)
			errs = append(errs, err)
		}
	}
	p.changes = pending
	if len(pending) == 0 {
		p.active = false
	}
	return errors.Join(errs...)
}
