package plugin

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeTimeoutKeepsSlotUntilOperationExits(t *testing.T) {
	state := &pluginRuntimeState{semaphore: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	defer finish()
	result := make(chan error, 1)
	go func() {
		result <- state.run(ctx, func(context.Context) error {
			close(started)
			<-release // Simulate host code that cannot immediately be interrupted.
			return nil
		})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("operation did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled caller did not return")
	}
	if len(state.semaphore) != 1 {
		t.Fatal("unfinished operation released its slot")
	}
	waitCtx, stopWait := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer stopWait()
	var called atomic.Bool
	err := state.run(waitCtx, func(context.Context) error { called.Store(true); return nil })
	if called.Load() || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued operation started=%v err=%v", called.Load(), err)
	}
	finish()
	retryCtx, stopRetry := context.WithTimeout(context.Background(), time.Second)
	defer stopRetry()
	if err := state.run(retryCtx, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("slot was not released after the operation exited: %v", err)
	}
}

func TestRuntimePanicReleasesSlot(t *testing.T) {
	state := &pluginRuntimeState{semaphore: make(chan struct{}, 1)}
	if err := state.run(context.Background(), func(context.Context) error { panic("test") }); err == nil {
		t.Fatal("expected plugin error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := state.run(ctx, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("slot was not reusable after panic: %v", err)
	}
}
