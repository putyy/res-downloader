package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
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
	settings SettingsProvider
	api      APIHandler
	proxy    ProxyProvider
	report   ErrorReporter
	server   *http.Server
	listener net.Listener
}

func New(settings SettingsProvider, api APIHandler, proxy ProxyProvider, report ErrorReporter) *Gateway {
	return &Gateway{settings: settings, api: api, proxy: proxy, report: report}
}

func (g *Gateway) Start() error {
	if g == nil {
		return fmt.Errorf("HTTP gateway is nil")
	}
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

func (g *Gateway) Close(ctx context.Context) error {
	if g == nil || g.server == nil {
		return nil
	}
	err := g.server.Shutdown(ctx)
	g.server = nil
	g.listener = nil
	return err
}

func (g *Gateway) Active() bool {
	return g != nil && g.server != nil && g.listener != nil
}

func (g *Gateway) Address() string {
	if g == nil || g.listener == nil {
		return ""
	}
	return g.listener.Addr().String()
}
