package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"res-downloader/internal/config"
	"res-downloader/internal/download"
	"res-downloader/internal/logging"
	"res-downloader/internal/media"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	captureproxy "res-downloader/internal/proxy"
	"res-downloader/internal/resource"
	internalserver "res-downloader/internal/server"
	"sync"
	"time"
)

type respData map[string]interface{}

type ResponseData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func NewResponse(code int, message string, data interface{}) *ResponseData {
	return &ResponseData{Code: code, Message: message, Data: data}
}

type pendingPluginArchive struct {
	data      []byte
	manifest  shared.PluginManifest
	digest    string
	expiresAt time.Time
}

// Host contains the desktop-only capabilities used by the HTTP API. Keeping
// these callbacks here prevents the transport package from depending on the
// Wails application composition root.
type Host struct {
	Context                    func() context.Context
	AppInfo                    interface{}
	PublicCertificate          func() []byte
	InstallCertificateWithPass func(string) (string, error)
	UninstallCertificate       func(string) (string, error)
	CertificateStatus          func() interface{}
	RetryCertificateCleanup    func(string) interface{}
	OpenSystemProxy            func(string) error
	UnsetSystemProxy           func(string) error
	ProxyEnabled               func() bool
}

func (h Host) context() context.Context {
	if h.Context != nil {
		if ctx := h.Context(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func (h Host) proxyEnabled() bool {
	return h.ProxyEnabled != nil && h.ProxyEnabled()
}

type Server struct {
	pluginArchiveMu sync.Mutex
	pluginArchives  map[string]pendingPluginArchive
	hlsPreviewMu    sync.Mutex
	hlsPreviews     map[string]hlsPreviewTarget
	previewClientMu sync.Mutex
	previewClient   *http.Client
	host            Host
	config          *config.Config
	proxy           *captureproxy.Engine
	resources       *resource.Resource
	plugins         *plugin.PluginManager
	downloads       *download.Scheduler
	media           *media.Engine
	logger          *logging.Logger
	gateway         *internalserver.Gateway
	sessionToken    string
}

func New(
	host Host,
	sessionToken string,
	config *config.Config,
	proxy *captureproxy.Engine,
	resources *resource.Resource,
	plugins *plugin.PluginManager,
	downloads *download.Scheduler,
	media *media.Engine,
	logger *logging.Logger,
) *Server {
	h := &Server{
		pluginArchives: make(map[string]pendingPluginArchive), hlsPreviews: make(map[string]hlsPreviewTarget),
		host: host, config: config,
		proxy: proxy, resources: resources, plugins: plugins, downloads: downloads, media: media,
		logger: logger, sessionToken: sessionToken,
	}
	h.gateway = internalserver.New(
		func() internalserver.Settings {
			snapshot := config.Snapshot()
			return internalserver.Settings{Host: snapshot.Host, Port: snapshot.Port}
		},
		h.HandleAPI,
		func() http.Handler {
			if proxy == nil {
				return nil
			}
			return proxy.Proxy
		},
		func(err error) { logger.Esg(err, "HTTP service stopped unexpectedly") },
	)
	return h
}

func (h *Server) SessionToken() string {
	if h == nil {
		return ""
	}
	return h.sessionToken
}

func (h *Server) Start() error {
	if err := h.gateway.Start(); err != nil {
		return err
	}
	snapshot := h.config.Snapshot()
	fmt.Println("Service started, listening http://" + snapshot.Host + ":" + snapshot.Port)
	return nil
}

func (h *Server) Close(ctx context.Context) error {
	return h.gateway.Close(ctx)
}

func (h *Server) Active() bool {
	return h != nil && h.gateway != nil && h.gateway.Active()
}

func (h *Server) writeJSON(w http.ResponseWriter, data *ResponseData) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Err(err)
	}
}

func (h *Server) error(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}
	if len(args) > 0 {
		message = args[0].(string)
	}
	if len(args) > 1 {
		data = args[1]
	}
	h.writeJSON(w, NewResponse(0, message, data))
}

func (h *Server) success(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}
	if len(args) > 0 {
		data = args[0]
	}
	if len(args) > 1 {
		message = args[1].(string)
	}
	h.writeJSON(w, NewResponse(1, message, data))
}
