package app

import (
	"errors"
	"net"
	"runtime"
	"syscall"
	"testing"
	"time"

	"res-downloader/internal/httpapi"
)

func TestAPISessionReportsStartupFailure(t *testing.T) {
	startupErr := errors.New("start HTTP service: listen tcp 127.0.0.1:8899: address already in use")
	runtime := &Runtime{startupDone: make(chan struct{}), startupErr: startupErr}
	close(runtime.startupDone)

	// No HTTP server is needed: startup failure must be returned before a
	// session can be issued for a listener owned by another process.
	response := NewBind(runtime).APISession()
	if response.Code != 0 || response.Message != startupErr.Error() {
		t.Fatalf("startup error was lost: %+v", response)
	}
	data, ok := response.Data.(map[string]bool)
	if !ok || !data["restartRequired"] {
		t.Fatalf("missing restart guidance: %+v", response.Data)
	}
}

func TestAppInfoWaitsForCertificateInitialisation(t *testing.T) {
	r := &Runtime{App: &App{Version: "4.0.0"}, startupDone: make(chan struct{})}
	result := make(chan *httpapi.ResponseData, 1)
	go func() { result <- NewBind(r).AppInfo() }()
	select {
	case <-result:
		t.Fatal("application info returned before initialisation finished")
	case <-time.After(20 * time.Millisecond):
	}
	r.App.CertificateError = "certificate unavailable"
	r.startupErr = errors.New("startup failed")
	close(r.startupDone)
	response := <-result
	if response.Code != 1 || response.Data.(*App).CertificateError != "certificate unavailable" {
		t.Fatalf("application info did not include final startup state: %+v", response)
	}
	// Startup failure remains available through APISession, including port recovery.
	if response := NewBind(r).APISession(); response.Code != 0 {
		t.Fatal("startup failure was hidden")
	}
}

func TestAPISessionOffersPortRecoveryOnlyForBindFailure(t *testing.T) {
	busy := syscall.EADDRINUSE
	if runtime.GOOS == "windows" {
		busy = syscall.Errno(10048)
	}
	for _, test := range []struct {
		err  error
		want bool
	}{
		{&net.OpError{Op: "listen", Net: "tcp", Err: busy}, true},
		{errors.New("start capture proxy: certificate is unavailable"), false},
		{errors.New("HTTP 401"), false},
	} {
		runtime := &Runtime{startupDone: make(chan struct{}), startupErr: test.err}
		close(runtime.startupDone)
		response := NewBind(runtime).APISession()
		if data := response.Data.(map[string]bool); data["portUnavailable"] != test.want {
			t.Fatalf("error %v: port recovery = %v, want %v", test.err, data["portUnavailable"], test.want)
		}
	}
}

func TestPortRestartRejectsOtherStartupStates(t *testing.T) {
	for _, test := range []struct {
		name string
		done bool
		err  error
	}{
		{"still starting", false, nil},
		{"started normally", true, nil},
		{"unrelated failure", true, errors.New("certificate is unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime := &Runtime{App: &App{}, startupDone: make(chan struct{}), startupErr: test.err}
			runtime.App.runtime = runtime
			if test.done {
				close(runtime.startupDone)
			}
			if err := runtime.App.preparePortRestart(); err == nil || runtime.App.portRestart != nil {
				t.Fatalf("unexpected port restart: %v", err)
			}
		})
	}
}

func TestAPISessionWaitsForStartupResult(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "ready"
		if failed {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) {
			runtime := &Runtime{
				startupDone: make(chan struct{}),
				HTTP:        httpapi.New(httpapi.Host{}, "test-session", nil, nil, nil, nil, nil, nil, nil),
			}
			defer func() {
				select {
				case <-runtime.startupDone:
				default:
					close(runtime.startupDone)
				}
			}()
			result := make(chan *httpapi.ResponseData, 1)
			go func() { result <- NewBind(runtime).APISession() }()
			select {
			case response := <-result:
				t.Fatalf("returned a session before startup completed: %+v", response)
			case <-time.After(20 * time.Millisecond):
			}
			if failed {
				runtime.startupErr = errors.New("start capture proxy: unavailable")
			}
			close(runtime.startupDone)
			response := <-result
			if failed {
				if response.Code != 0 || response.Message != runtime.startupErr.Error() {
					t.Fatalf("startup failure was ignored: %+v", response)
				}
				return
			}
			data, ok := response.Data.(map[string]string)
			if response.Code != 1 || !ok || data["token"] != "test-session" {
				t.Fatalf("unexpected session: %+v", response)
			}
		})
	}
}
