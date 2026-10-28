package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type Settings struct {
	Host string
	Port string
}

type SettingsProvider func() Settings
type APIHandler func(http.ResponseWriter, *http.Request) bool
type ErrorReporter func(error)
type ProxyProvider func() http.Handler

// Gateway owns the HTTP listener lifecycle and dispatches requests either to
// the local API or to the capture proxy. Business handlers remain outside of
// this package.
type Gateway struct {
	// mu guards server and listener: Start and Close run on the lifecycle
	// goroutine while Active and Address are polled by the frontend.
	mu       sync.RWMutex
	settings SettingsProvider
	api      APIHandler
	proxy    ProxyProvider
	report   ErrorReporter
	server   *http.Server
	listener net.Listener
}

// New returns a gateway that has not opened a listener yet. All callbacks are
// optional except settings, which Start calls once to resolve the bind address.
func New(settings SettingsProvider, api APIHandler, proxy ProxyProvider, report ErrorReporter) *Gateway {
	return &Gateway{settings: settings, api: api, proxy: proxy, report: report}
}

// Start binds the configured Host:Port and serves API and proxy traffic on the
// same listener. It is a no-op when the gateway is already running and returns
// the bind error when the address is unavailable.
func (g *Gateway) Start() error {
	if g == nil {
		return fmt.Errorf("HTTP gateway is nil")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.server != nil {
		return nil
	}
	settings := g.settings()
	listener, err := net.Listen("tcp", net.JoinHostPort(settings.Host, settings.Port))
	if err != nil {
		return err
	}
	g.listener = listener
	server := &http.Server{
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host == net.JoinHostPort("127.0.0.1", settings.Port) && g.api != nil && g.api(w, r) {
				return
			}
			var proxy http.Handler
			if g.proxy != nil {
				proxy = g.proxy()
			}
			if proxy == nil {
				http.Error(w, "capture proxy is unavailable", http.StatusServiceUnavailable)
				return
			}
			proxy.ServeHTTP(w, r)
		}),
	}
	g.server = server
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed && g.report != nil {
			g.report(serveErr)
		}
	}()
	return nil
}

// Close stops accepting new connections and waits for in-flight requests until
// ctx expires. It is safe to call more than once.
func (g *Gateway) Close(ctx context.Context) error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	server := g.server
	g.server = nil
	g.listener = nil
	g.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

// Active reports whether a listener is currently bound.
func (g *Gateway) Active() bool {
	if g == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.server != nil && g.listener != nil
}

// Address returns the bound listener address, or an empty string when the
// gateway is not running.
func (g *Gateway) Address() string {
	if g == nil {
		return ""
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.listener == nil {
		return ""
	}
	return g.listener.Addr().String()
}
