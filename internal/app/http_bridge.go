package app

import (
	"context"
	"res-downloader/internal/httpapi"
	desktopsystem "res-downloader/internal/system"
)

type HttpServer = httpapi.Server

func NewHTTPServer(
	app *App,
	sessionToken string,
	config *Config,
	proxy *Proxy,
	resources *Resource,
	plugins *PluginManager,
	downloads *DownloadScheduler,
	media *mediaEngine,
	system *SystemSetup,
	logger *Logger,
) *HttpServer {
	host := httpapi.Host{
		Context: func() context.Context {
			return app.ctx
		},
		AppInfo: app,
		PublicCertificate: func() []byte {
			return app.PublicCrt
		},
		InstallCertificateWithPass: app.installCertWithPassword,
		UninstallCertificate:       app.uninstallCertWithPassword,
		CertificateStatus: func() interface{} {
			return desktopsystem.CurrentCertificateStatus(app.PublicCrt, app.CertificateError, system)
		},
		RetryCertificateCleanup: func(password string) interface{} {
			return desktopsystem.RetryLegacyCertificateMigration(system, password)
		},
		OpenSystemProxy:  app.OpenSystemProxy,
		UnsetSystemProxy: app.UnsetSystemProxy,
		ProxyEnabled: func() bool {
			return app.IsProxy
		},
	}
	return httpapi.New(host, sessionToken, config, proxy, resources, plugins, downloads, media, logger)
}
