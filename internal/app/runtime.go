package app

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"res-downloader/internal/capture"
	"res-downloader/internal/events"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	desktopsystem "res-downloader/internal/system"
)

// Runtime is the application composition root. Construction, startup and
// shutdown are deliberately separate so constructors do not open listeners or
// start background work as a side effect.
type Runtime struct {
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
}

func NewRuntime(assets embed.FS, wailsConfig string) (*Runtime, error) {
	app, err := newApp(assets, wailsConfig)
	if err != nil {
		return nil, err
	}

	logger := newAppLogger(app)
	eventEmitter := events.New()
	app.events = eventEmitter

	certificate, privateKey, err := desktopsystem.InitializeCertificateAuthority(app.UserDir)
	if err != nil {
		app.CertificateError = err.Error()
		logger.Esg(err, "initialise device certificate authority")
	} else {
		app.PublicCrt, app.PrivateKey = certificate, privateKey
	}
	config := newConfig(app, logger)
	captures, err := capture.New(filepath.Join(app.UserDir, "capture-cache"))
	if err != nil {
		return nil, fmt.Errorf("initialise response capture cache: %w", err)
	}
	media := NewMediaEngine(config)
	system := newSystemSetup(app, config, logger)
	resources := newResource(app, config, media, logger)
	resources.SetCaptureSource(captures)
	initialConfig := config.Snapshot()
	rules, err := newRuleSet(initialConfig.InterceptionPolicies)
	if err != nil {
		return nil, fmt.Errorf("initialise interception policies: %w", err)
	}
	plugins := plugin.NewManager(app.UserDir, func() plugin.NetworkSettings {
		snapshot := config.Snapshot()
		return plugin.NetworkSettings{DownloadProxy: snapshot.DownloadProxy, UpstreamProxy: snapshot.UpstreamProxy, Port: snapshot.Port}
	}, media, resources, logger)
	plugins.SetCaptureStore(captures)
	resources.ReconcilePluginAvailability(plugins)
	downloads := newDownloadScheduler(app, config, resources, plugins, logger)
	plugins.SetPageDownloadHandler(func(candidate shared.ResourceCandidate) error {
		_, err := downloads.Enqueue(candidate)
		return err
	})
	resources.SetPlugins(plugins)
	resources.SetDownloads(downloads)
	proxy := NewProxy(app, config, rules, plugins, logger)
	proxy.SetCaptureStore(captures)
	sessionToken, err := newAPISessionToken()
	if err != nil {
		return nil, fmt.Errorf("initialise API session: %w", err)
	}
	httpServer := NewHTTPServer(app, sessionToken, config, proxy, resources, plugins, downloads, media, system, logger)

	runtime := &Runtime{
		App: app, Config: config, Logger: logger, System: system, Rules: rules,
		Resources: resources, Plugins: plugins, Downloads: downloads, Media: media, Events: eventEmitter,
		Proxy: proxy, HTTP: httpServer,
		Captures: captures,
	}
	app.runtime = runtime
	config.SetApplyHook(func(previous, current Config) error {
		if previous.UpstreamProxy != current.UpstreamProxy || previous.OpenProxy != current.OpenProxy {
			proxy.SetTransport()
		}
		downloads.SetWorkerCount(current.DownNumber)
		return rules.Load(current.InterceptionPolicies)
	})
	return runtime, nil
}

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
	r.App.ctx = ctx
	r.Events.SetContext(ctx)
	if err := r.Proxy.Start(); err != nil {
		return fmt.Errorf("start capture proxy: %w", err)
	}
	if err := r.HTTP.Start(); err != nil {
		return fmt.Errorf("start HTTP service: %w", err)
	}
	r.Downloads.Start()
	return nil
}

func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	var first error
	if err := r.App.UnsetSystemProxy(""); err != nil {
		first = err
	}
	if err := r.HTTP.Close(ctx); err != nil && first == nil {
		first = err
	}
	if r.Downloads != nil {
		r.Downloads.Close()
	}
	if r.Resources != nil {
		r.Resources.Close()
	}
	if r.Captures != nil {
		_ = r.Captures.Close()
	}
	if r.Logger != nil {
		r.Logger.Close()
	}
	if r.App.IsReset {
		if err := r.App.ResetApp(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
