package app

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"res-downloader/internal/capture"
	"res-downloader/internal/control"
	"res-downloader/internal/events"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	desktopsystem "res-downloader/internal/system"
	"res-downloader/internal/updates"
	"strings"
	"sync"
	"time"
)

// Runtime is the application composition root. Construction, startup and
// shutdown are separate so construction does not open business databases,
// initialise plugins, or start listeners and background tasks.
type Runtime struct {
	lifecycleMu   sync.Mutex
	startupMu     sync.Mutex
	startupCancel context.CancelFunc
	stopping      bool
	startupOnce   sync.Once
	startupDone   chan struct{}
	startupErr    error
	closeOnce     sync.Once
	closeErr      error

	Updates   *updates.Manager
	App       *App
	Config    *Config
	Logger    *Logger
	System    *SystemSetup
	Rules     *RuleSet
	Resources *Resource
	Plugins   *PluginManager
	Downloads *DownloadScheduler
	Media     *mediaEngine
	Events    *events.Emitter
	Proxy     *Proxy
	HTTP      *HttpServer
	Captures  *capture.Store
	Control   *control.Server
}

func NewRuntime(assets embed.FS, wailsConfig string) (*Runtime, error) {
	app, err := newApp(assets, wailsConfig)
	if err != nil {
		return nil, err
	}
	return prepareRuntime(app), nil
}

func prepareRuntime(app *App) *Runtime {
	logger := newAppLogger(app)
	eventEmitter := events.New()
	app.events = eventEmitter
	config := newConfig(app, logger)
	r := &Runtime{
		startupDone: make(chan struct{}),
		App:         app, Config: config, Logger: logger, Events: eventEmitter,
	}
	app.runtime = r
	return r
}

// initialise runs from OnDomReady, after Wails has checked its single-instance
// lock. A duplicate desktop launch must not open databases or mutate plugins.
func (r *Runtime) initialise(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	app, config, logger := r.App, r.Config, r.Logger
	if err := removeLegacyLocalFiles(app.UserDir); err != nil {
		return fmt.Errorf("remove legacy local files: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	certificate, privateKey, err := desktopsystem.InitializeCertificateAuthority(app.UserDir)
	if err != nil {
		app.CertificateError = err.Error()
		logger.Esg(err, "initialise device certificate authority")
	} else {
		app.PublicCrt, app.PrivateKey = certificate, privateKey
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	captures, err := capture.New(filepath.Join(app.UserDir, "capture-cache"))
	if err != nil {
		return fmt.Errorf("initialise response capture cache: %w", err)
	}
	r.Captures = captures
	if err := ctx.Err(); err != nil {
		return err
	}
	media := NewMediaEngine(config)
	system := newSystemSetup(app, config, logger)
	resources := newResource(app, config, media, logger)
	r.Media, r.System, r.Resources = media, system, resources
	resources.SetCaptureSource(captures)
	if err := ctx.Err(); err != nil {
		return err
	}
	initialConfig := config.Snapshot()
	rules, err := newRuleSet(initialConfig.InterceptionPolicies)
	if err != nil {
		return fmt.Errorf("initialise interception policies: %w", err)
	}
	plugins := plugin.NewManager(app.UserDir, func() plugin.NetworkSettings {
		snapshot := config.Snapshot()
		return plugin.NetworkSettings{DownloadProxy: snapshot.DownloadProxy, UpstreamProxy: snapshot.UpstreamProxy, Port: snapshot.Port}
	}, media, resources, logger)
	plugins.SetCaptureStore(captures)
	r.Rules, r.Plugins = rules, plugins
	if err := ctx.Err(); err != nil {
		return err
	}
	resources.ReconcilePluginAvailability(plugins)
	downloads := newDownloadScheduler(app, config, resources, plugins, logger)
	r.Downloads = downloads
	if err := ctx.Err(); err != nil {
		return err
	}
	plugins.SetPageDownloadHandler(func(candidate shared.ResourceCandidate) error {
		_, err := downloads.Enqueue(candidate)
		return err
	})
	resources.SetPlugins(plugins)
	resources.SetDownloads(downloads)
	proxy := NewProxy(app, config, rules, plugins, logger)
	proxy.SetCaptureStore(captures)
	r.Proxy = proxy
	sessionToken, err := newAPISessionToken()
	if err != nil {
		return fmt.Errorf("initialise API session: %w", err)
	}
	r.HTTP = NewHTTPServer(app, sessionToken, config, proxy, resources, plugins, downloads, media, system, logger)
	r.Updates = updates.New(app.Version, app.UserDir, func() updates.Network {
		snapshot := config.Snapshot()
		return updates.Network{Port: snapshot.Port, Upstream: snapshot.UpstreamProxy, DownloadProxy: snapshot.DownloadProxy}
	})
	config.SetApplyHook(func(previous, current Config) error {
		if previous.UpstreamProxy != current.UpstreamProxy || previous.OpenProxy != current.OpenProxy {
			proxy.SetTransport()
		}
		downloads.SetWorkerCount(current.DownNumber)
		return rules.Load(current.InterceptionPolicies)
	})
	return ctx.Err()
}

const startupWaitTimeout = 30 * time.Second

func newAPISessionToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (r *Runtime) Start(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("runtime is nil")
	}
	r.startupOnce.Do(func() {
		r.lifecycleMu.Lock()
		defer r.lifecycleMu.Unlock()
		defer close(r.startupDone)
		startupCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		r.startupMu.Lock()
		r.startupCancel = cancel
		if r.stopping {
			cancel()
		}
		r.startupMu.Unlock()
		if err := startupCtx.Err(); err != nil {
			r.startupErr = err
			return
		}
		// Events retain the Wails context after the startup-only context ends.
		r.Events.SetContext(ctx)
		r.startupErr = r.start(startupCtx)
		if r.startupErr != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := r.closeServices(cleanupCtx); err != nil {
				r.Logger.Esg(err, "release resources after startup failure")
			}
			cancel()
		}
	})
	return r.startupErr
}

func (r *Runtime) start(ctx context.Context) error {
	if err := r.initialise(ctx); err != nil {
		return err
	}
	if err := r.Proxy.Start(); err != nil {
		return fmt.Errorf("start capture proxy: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.HTTP.Start(); err != nil {
		return fmt.Errorf("start HTTP service: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.Downloads.Start()
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.Control == nil {
		server, err := control.Start(r.App.UserDir, r.HTTP.ControlHandler, func(err error) {
			r.Logger.Esg(err, "local automation service stopped unexpectedly")
		})
		if err != nil {
			// Automation is optional; a discovery failure must not stop capture
			// or downloads in an otherwise working desktop application.
			r.Logger.Esg(err, "start local automation service")
		} else {
			r.Control = server
		}
	}
	return ctx.Err()
}

// Stop between initialization stages, while retaining the lifecycle lock until
// the current synchronous operation finishes and can be cleaned up safely.
func (r *Runtime) cancelStartup() {
	r.startupMu.Lock()
	defer r.startupMu.Unlock()
	r.stopping = true
	if r.startupCancel != nil {
		r.startupCancel()
	}
}

func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.cancelStartup()
	// Resource cleanup is idempotent; App owns reset, restart and update handoff.
	r.closeOnce.Do(func() { r.closeErr = r.close(ctx) })
	return r.closeErr
}

func (r *Runtime) close(ctx context.Context) error {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	return r.closeServices(ctx)
}

// Also used on partial startup failure; a later Close remains safe.
func (r *Runtime) closeServices(ctx context.Context) error {
	if r.Updates != nil {
		r.Updates.Close()
	}
	var first error
	r.logShutdownStage("close automation service")
	if err := r.Control.Close(ctx); err != nil {
		first = fmt.Errorf("close automation service: %w", err)
	}
	r.logShutdownStage("disable system proxy")
	if err := r.App.UnsetSystemProxy(""); err != nil {
		if first == nil {
			first = fmt.Errorf("disable system proxy: %w", err)
		}
	}
	r.logShutdownStage("close HTTP gateway")
	if r.HTTP != nil {
		if err := r.HTTP.Close(ctx); err != nil && first == nil {
			first = fmt.Errorf("close HTTP gateway: %w", err)
		}
	}
	r.logShutdownStage("close download scheduler")
	if r.Downloads != nil {
		r.Downloads.Close()
	}
	r.logShutdownStage("close resource database and capture cache")
	if r.Resources != nil {
		r.Resources.Close()
	}
	if r.Captures != nil {
		if err := r.Captures.Close(); err != nil && first == nil {
			first = fmt.Errorf("close capture cache: %w", err)
		}
	}
	return first
}

// Await construction and startup without blocking embedded frontend assets.
func (r *Runtime) awaitStartup(ctx context.Context) error {
	select {
	case <-r.startupDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runtime) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, "/api") {
			next.ServeHTTP(w, request)
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), startupWaitTimeout)
		defer cancel()
		if err := r.awaitStartup(ctx); err != nil {
			http.Error(w, "Application services are still starting", http.StatusServiceUnavailable)
			return
		}
		if r.startupErr != nil || r.HTTP == nil {
			http.Error(w, "Application services are unavailable", http.StatusServiceUnavailable)
			return
		}
		if !r.HTTP.HandleAPI(w, request) {
			next.ServeHTTP(w, request)
		}
	})
}

func (r *Runtime) logShutdownStage(stage string) {
	if r.Logger != nil {
		r.Logger.Info().Str("stage", stage).Msg("application shutdown")
	}
}
