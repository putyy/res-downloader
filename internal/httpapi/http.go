package httpapi

import (
	"context"
	"encoding/json"
<<<<<<< Updated upstream
=======
	"errors"
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
// NewResponse wraps data in the envelope shared by every endpoint: code 1 for
// success and 0 for failure, plus a human readable message.
>>>>>>> Stashed changes
func NewResponse(code int, message string, data interface{}) *ResponseData {
	return &ResponseData{Code: code, Message: message, Data: data}
}

type pendingPluginArchive struct {
	data      []byte
	manifest  shared.PluginManifest
	digest    string
	expiresAt time.Time
}

<<<<<<< Updated upstream
=======
// FileFilter narrows a native file dialog to matching files.
type FileFilter struct {
	DisplayName string
	Pattern     string
}

>>>>>>> Stashed changes
// Host contains the desktop-only capabilities used by the HTTP API. Keeping
// these callbacks here prevents the transport package from depending on the
// Wails application composition root.
type Host struct {
	Update                     func(context.Context, string, bool) (interface{}, error)
	Context                    func() context.Context
<<<<<<< Updated upstream
=======
	OpenFileDialog             func(title string, filters []FileFilter) (string, error)
	OpenDirectoryDialog        func(title string) (string, error)
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
func (h Host) openFile(title string, filters []FileFilter) (string, error) {
	if h.OpenFileDialog == nil {
		return "", errors.New("native file dialog is unavailable")
	}
	return h.OpenFileDialog(title, filters)
}

func (h Host) openDirectory(title string) (string, error) {
	if h.OpenDirectoryDialog == nil {
		return "", errors.New("native directory dialog is unavailable")
	}
	return h.OpenDirectoryDialog(title)
}

>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
// New builds the API server and its gateway. sessionToken is the per-launch
// bearer token clients must present; the gateway reads Host and Port from config
// on every start so a port change does not need a new server.
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
// SessionToken returns the bearer token issued for this launch. The desktop
// frontend receives it through the Wails binding; it is empty on a nil server.
>>>>>>> Stashed changes
func (h *Server) SessionToken() string {
	if h == nil {
		return ""
	}
	return h.sessionToken
}

<<<<<<< Updated upstream
=======
// Start binds the gateway and prints the effective listen address.
>>>>>>> Stashed changes
func (h *Server) Start() error {
	if err := h.gateway.Start(); err != nil {
		return err
	}
	snapshot := h.config.Snapshot()
	fmt.Println("Service started, listening http://" + snapshot.Host + ":" + snapshot.Port)
	return nil
}

<<<<<<< Updated upstream
=======
// Close shuts the gateway down within ctx.
>>>>>>> Stashed changes
func (h *Server) Close(ctx context.Context) error {
	return h.gateway.Close(ctx)
}

<<<<<<< Updated upstream
=======
// Active reports whether the gateway currently has a bound listener.
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
// error writes a failure response. The optional first argument is the message
// and the optional second argument is the payload returned to the caller.
// Non-string messages are stringified instead of panicking on a bad call site.
>>>>>>> Stashed changes
func (h *Server) error(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}
	if len(args) > 0 {
<<<<<<< Updated upstream
		message = args[0].(string)
=======
		message = responseMessage(args[0])
>>>>>>> Stashed changes
	}
	if len(args) > 1 {
		data = args[1]
	}
	h.writeJSON(w, NewResponse(0, message, data))
}

<<<<<<< Updated upstream
=======
// success writes a success response. The optional first argument is the payload
// and the optional second argument is the message.
>>>>>>> Stashed changes
func (h *Server) success(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}
	if len(args) > 0 {
		data = args[0]
	}
	if len(args) > 1 {
<<<<<<< Updated upstream
		message = args[1].(string)
	}
	h.writeJSON(w, NewResponse(1, message, data))
}
=======
		message = responseMessage(args[1])
	}
	h.writeJSON(w, NewResponse(1, message, data))
}

// responseMessage accepts the string messages used by every handler and falls
// back to a formatted value so a mistyped argument cannot panic the API server.
func responseMessage(value interface{}) string {
	switch message := value.(type) {
	case string:
		return message
	case nil:
		return "ok"
	case error:
		return message.Error()
	default:
		return fmt.Sprint(message)
	}
}
>>>>>>> Stashed changes
