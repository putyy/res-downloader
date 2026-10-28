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
	pluginRuntimeTimeout     = 10 * time.Second
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

	callCtx, cancel := context.WithTimeout(ctx, pluginRuntimeTimeout)
	defer cancel()
	select {
	case s.semaphore <- struct{}{}:
	case <-callCtx.Done():
		return pluginCallContextError(callCtx)
	}
	// A cancelled waiter must not start work even if a slot became available.
	if callCtx.Err() != nil {
		<-s.semaphore
		return pluginCallContextError(callCtx)
	}
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
			// A timeout only releases the caller. Keep the slot until the actual
			// operation exits, including operations that ignore cancellation.
			<-s.semaphore
			done <- callErr
		}()
		callErr = operation(callCtx)
	}()
	select {
	case err = <-done:
		if callCtx.Err() != nil {
			return pluginCallContextError(callCtx)
		}
		return err
	case <-callCtx.Done():
		return pluginCallContextError(callCtx)
	}
}

func pluginCallContextError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("plugin operation timed out: %w", ctx.Err())
	}
	return ctx.Err()
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
