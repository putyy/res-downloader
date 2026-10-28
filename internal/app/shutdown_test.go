package app

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

func TestAppShutdownConcurrentUpdateAndWailsCallbacks(t *testing.T) {
	var output bytes.Buffer
	a := &App{}
	a.runtime = &Runtime{App: a, Logger: &Logger{Logger: zerolog.New(&output)}}
	var callers sync.WaitGroup
	for range 8 {
		callers.Go(func() {
			if err := a.shutdown(); err != nil {
				t.Errorf("shutdown: %v", err)
			}
		})
	}
	callers.Wait()
	if got := strings.Count(output.String(), `"message":"application shutdown completed"`); got != 1 {
		t.Fatalf("shutdown completed %d times; want once", got)
	}
	if !a.closing || !a.windowClosed {
		t.Fatal("shutdown left the application open to new actions")
	}
}

func TestRuntimeCloseLeavesExitActionsToApp(t *testing.T) {
	a := &App{UserDir: t.TempDir(), IsReset: true}
	a.runtime = &Runtime{App: a}
	sentinel := filepath.Join(a.UserDir, "config.json")
	if err := os.WriteFile(sentinel, []byte("keep on failure"), 0600); err != nil {
		t.Fatal(err)
	}
	closeErr := errors.New("reserved port close failed")
	listener := &shutdownTestListener{err: closeErr}
	a.portRestart = &portRestart{listener: listener}
	if err := a.runtime.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if listener.closes != 0 || a.portRestart == nil {
		t.Fatal("Runtime.Close performed an application restart action")
	}
	// No command is supplied: failed cleanup must prevent reset/relaunch.
	for range 2 {
		if err := a.shutdown(); !errors.Is(err, closeErr) {
			t.Fatalf("shutdown failure was lost: %v", err)
		}
	}
	if listener.closes != 1 {
		t.Fatalf("reserved port released %d times", listener.closes)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("application state was removed after failed cleanup: %v", err)
	}
}

type shutdownTestListener struct {
	closes int
	err    error
}

func (*shutdownTestListener) Accept() (net.Conn, error) { panic("unused") }
func (*shutdownTestListener) Addr() net.Addr            { return &net.TCPAddr{} }
func (l *shutdownTestListener) Close() error {
	l.closes++
	return l.err
}
