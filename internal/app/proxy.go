package app

import internalproxy "res-downloader/internal/proxy"

type Proxy = internalproxy.Engine

func NewProxy(app *App, config *Config, rules *RuleSet, plugins *PluginManager, logger *Logger) *Proxy {
	proxy := internalproxy.New(
		func() ([]byte, []byte) {
			return app.PublicCrt, app.PrivateKey
		},
		func() internalproxy.Settings {
			snapshot := config.Snapshot()
			return internalproxy.Settings{
				UpstreamProxy: snapshot.UpstreamProxy,
				Port:          snapshot.Port,
				OpenProxy:     snapshot.OpenProxy,
			}
		},
		rules,
		plugins,
	)
	configureDevelopmentResponseDump(proxy, logger)
	return proxy
}
