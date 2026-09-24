package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"syscall"
	"testing"
)

func busyPortError() error {
	code := syscall.EADDRINUSE
	if runtime.GOOS == "windows" {
		code = syscall.Errno(10048)
	}
	return &net.OpError{Op: "listen", Net: "tcp", Err: &os.SyscallError{Syscall: "bind", Err: code}}
}

func TestIsPortUnavailable(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{"wrapped bind error", fmt.Errorf("start HTTP service: %w", busyPortError()), true},
		{"plain message", errors.New("address already in use"), false},
		{"HTTP authentication", errors.New("HTTP 401"), false},
		{"connect error", &net.OpError{Op: "dial", Net: "tcp", Err: busyPortError()}, false},
		{"invalid address", &net.OpError{Op: "listen", Net: "tcp", Err: syscall.EINVAL}, false},
		{"no error", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := IsPortUnavailable(test.err); got != test.want {
				t.Fatalf("IsPortUnavailable(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
	if runtime.GOOS == "windows" {
		err := &net.OpError{Op: "listen", Net: "tcp", Err: &os.SyscallError{Syscall: "bind", Err: syscall.Errno(10013)}}
		if !IsPortUnavailable(err) {
			t.Fatal("Windows exclusive-use/access-denied bind error was not recognized")
		}
	}
}

type reservedTestListener struct {
	port   int
	closed bool
}

func (l *reservedTestListener) Accept() (net.Conn, error) { return nil, errors.New("not serving") }
func (l *reservedTestListener) Close() error              { l.closed = true; return nil }
func (l *reservedTestListener) Addr() net.Addr            { return &net.TCPAddr{Port: l.port} }

func TestReserveAlternativePortOrder(t *testing.T) {
	for _, test := range []struct {
		name       string
		failedPort string
		available  string
		wantCalls  []string
	}{
		{"custom port does not retry default", "9000", "18899", []string{"18899"}},
		{"default port failed", "8899", "18899", []string{"18899"}},
		{"skip failed candidate", "18899", "7788", []string{"7788"}},
		{"second candidate", "8899", "7788", []string{"18899", "7788"}},
		{"system allocated", "8899", "0", []string{"18899", "7788", "0"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls []string
			reserved := &reservedTestListener{}
			listener, port, err := reserveAlternativePort("::1", test.failedPort, func(network, address string) (net.Listener, error) {
				host, port, err := net.SplitHostPort(address)
				if err != nil || host != "::1" || network != "tcp" {
					t.Fatalf("invalid listen address: %s %s", network, address)
				}
				calls = append(calls, port)
				if port != test.available {
					return nil, busyPortError()
				}
				reserved.port, _ = strconv.Atoi(port)
				if port == "0" {
					reserved.port = 49152
				}
				return reserved, nil
			})
			if err != nil || listener != reserved || reserved.closed || port != strconv.Itoa(reserved.port) {
				t.Fatalf("port was not reserved: listener=%v port=%s err=%v", listener, port, err)
			}
			if !reflect.DeepEqual(calls, test.wantCalls) {
				t.Fatalf("attempts = %v, want %v", calls, test.wantCalls)
			}
		})
	}
}

func TestReserveAlternativePortFailures(t *testing.T) {
	for _, test := range []struct {
		name  string
		err   error
		calls int
	}{
		{"all unavailable", busyPortError(), 5},
		{"unrelated error", &net.OpError{Op: "listen", Net: "tcp", Err: syscall.EINVAL}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			listener, port, err := reserveAlternativePort("127.0.0.1", "8899", func(string, string) (net.Listener, error) {
				calls++
				return nil, test.err
			})
			if listener != nil || port != "" || !errors.Is(err, test.err) || calls != test.calls {
				t.Fatalf("unexpected failure: listener=%v port=%s err=%v calls=%d", listener, port, err, calls)
			}
		})
	}
}

func TestReserveAlternativePortRejectsInvalidAllocation(t *testing.T) {
	var rejected []*reservedTestListener
	_, _, err := reserveAlternativePort("127.0.0.1", "8899", func(string, string) (net.Listener, error) {
		listener := &reservedTestListener{port: 65535}
		rejected = append(rejected, listener)
		return listener, nil
	})
	if err == nil {
		t.Fatal("out-of-range port was accepted")
	}
	for _, listener := range rejected {
		if !listener.closed {
			t.Fatal("rejected port reservation was leaked")
		}
	}
}
