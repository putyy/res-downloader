package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"res-downloader/core/plugins"
	"res-downloader/core/shared"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
)

type Proxy struct {
	ctx   context.Context
	Proxy *goproxy.ProxyHttpServer
	Is    bool
}

var pluginRegistry = make(map[string]shared.Plugin)

func init() {
	ps := []shared.Plugin{
		&plugins.QqPlugin{},
		&plugins.DefaultPlugin{},
	}

	bridge := &shared.Bridge{
		GetVersion: func() string {
			return appOnce.Version
		},
		GetResType: func(key string) (bool, bool) {
			return resourceOnce.getResType(key)
		},
		TypeSuffix: func(mine string) (string, string) {
			return globalConfig.typeSuffix(mine)
		},
		MediaIsMarked: func(key string) bool {
			return resourceOnce.mediaIsMarked(key)
		},
		MarkMedia: func(key string) {
			resourceOnce.markMedia(key)
		},
		GetConfig: func(key string) interface{} {
			return globalConfig.getConfig(key)
		},
		Send: func(t string, data interface{}) {
			httpServerOnce.send(t, data)
		},
	}

	for _, p := range ps {
		p.SetBridge(bridge)
		for _, domain := range p.Domains() {
			pluginRegistry[domain] = p
		}
	}
}

func initProxy() *Proxy {
	if proxyOnce == nil {
		proxyOnce = &Proxy{}
		proxyOnce.Startup()
	}
	return proxyOnce
}

func (p *Proxy) Startup() {
	err := p.setCa()
	if err != nil {
		DialogErr("Failed to start proxy service：" + err.Error())
		return
	}

	p.Proxy = goproxy.NewProxyHttpServer()
	//p.Proxy.KeepDestinationHeaders = true
	//p.Proxy.Verbose = false
	p.setTransport()
	//p.Proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	p.Proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if ruleOnce.shouldMitm(host) {
			return goproxy.MitmConnect, host
		}
		return goproxy.OkConnect, host
	})

	p.Proxy.OnRequest().DoFunc(p.httpRequestEvent)
	p.Proxy.OnResponse().DoFunc(p.httpResponseEvent)
}

func (p *Proxy) setCa() error {
	ca, err := tls.X509KeyPair(appOnce.PublicCrt, appOnce.PrivateKey)
	if err != nil {
		return err
	}
	if ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = ca
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	return nil
}

func (p *Proxy) setTransport() {
	transport := &http.Transport{
		DisableKeepAlives: false,
		// MaxIdleConnsPerHost: 10,
		DialContext: (&net.Dialer{
			Timeout: 60 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   60 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}

	p.Proxy.ConnectDial = nil
	p.Proxy.ConnectDialWithReq = nil

	if globalConfig.UpstreamProxy != "" && globalConfig.OpenProxy && !strings.Contains(globalConfig.UpstreamProxy, globalConfig.Port) {
		proxyURL, err := url.Parse(globalConfig.UpstreamProxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			p.Proxy.ConnectDial = p.Proxy.NewConnectDialToProxy(globalConfig.UpstreamProxy)
		}
	}
	p.Proxy.Tr = transport
}

func (p *Proxy) matchPlugin(host string) shared.Plugin {
	domain := shared.GetTopLevelDomain(host)
	if plugin, ok := pluginRegistry[domain]; ok {
		return plugin
	}
	return nil
}

func (p *Proxy) httpRequestEvent(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	plugin := p.matchPlugin(r.Host)
	if plugin != nil {
		newReq, newResp := plugin.OnRequest(r, ctx)
		if newResp != nil {
			return newReq, newResp
		}

		if newReq != nil {
			return newReq, nil
		}
	}
	return pluginRegistry["default"].OnRequest(r, ctx)
}

func (p *Proxy) httpResponseEvent(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.Request == nil {
		return resp
	}

	plugin := p.matchPlugin(resp.Request.Host)
	if plugin != nil {
		newResp := plugin.OnResponse(resp, ctx)
		if newResp != nil {
			return newResp
		}
	}

	return pluginRegistry["default"].OnResponse(resp, ctx)
}
