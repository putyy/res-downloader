package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	shared "res-downloader/internal/model"
)

const (
	pluginRuntimeConcurrency = 4
	pluginRuntimeTimeout     = 2 * time.Second
	pluginCircuitThreshold   = 3
	pluginCircuitPause       = 30 * time.Second
	pluginSlowCall           = 400 * time.Millisecond
)

type pluginRuntimeState struct {
	semaphore chan struct{}
	mu        sync.Mutex
	health    shared.PluginRuntimeHealth
}

func newPluginRuntimeState() *pluginRuntimeState {
	return &pluginRuntimeState{semaphore: make(chan struct{}, pluginRuntimeConcurrency)}
}

func (s *pluginRuntimeState) run(ctx context.Context, operation func(context.Context) error) (err error) {
	s.mu.Lock()
	pausedUntil := s.health.PausedUntil
	s.mu.Unlock()
	if pausedUntil > time.Now().UnixMilli() {
		return fmt.Errorf("plugin is temporarily paused after repeated failures")
	}

	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	callCtx, cancel := context.WithTimeout(ctx, pluginRuntimeTimeout)
	defer cancel()
	started := time.Now()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("plugin panic: %v", recovered)
		}
		duration := time.Since(started)
		s.mu.Lock()
		s.health.LastDurationMS = duration.Milliseconds()
		if duration >= pluginSlowCall {
			s.health.SlowCalls++
		}
		if err == nil {
			s.health.ConsecutiveErrors = 0
			s.health.LastError = ""
		} else {
			s.health.ConsecutiveErrors++
			s.health.TotalErrors++
			s.health.LastError = err.Error()
			if s.health.ConsecutiveErrors >= pluginCircuitThreshold {
				s.health.PausedUntil = time.Now().Add(pluginCircuitPause).UnixMilli()
			}
		}
		s.mu.Unlock()
	}()

	done := make(chan error, 1)
	go func() {
		var callErr error
		defer func() {
			if recovered := recover(); recovered != nil {
				callErr = fmt.Errorf("plugin panic: %v", recovered)
			}
			done <- callErr
		}()
		callErr = operation(callCtx)
	}()
	select {
	case err = <-done:
		return err
	case <-callCtx.Done():
		return errors.New("plugin operation timed out")
	}
}

func (s *pluginRuntimeState) snapshot() shared.PluginRuntimeHealth {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.health
}

func (m *PluginManager) runtimeState(id string) *pluginRuntimeState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runtimeStates == nil {
		m.runtimeStates = make(map[string]*pluginRuntimeState)
	}
	state := m.runtimeStates[id]
	if state == nil {
		state = newPluginRuntimeState()
		m.runtimeStates[id] = state
	}
	return state
}
