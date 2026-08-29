package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	shared "res-downloader/internal/model"
	"strconv"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
)

type Settings struct {
	UpstreamProxy string
	Port          string
	OpenProxy     bool
}

type CertificateProvider func() (certificatePEM, privateKeyPEM []byte)

type SettingsProvider func() Settings

type RuleMatcher interface {
	ShouldMITM(host string) bool
}

type PluginProcessor interface {
	BodyLimit(shared.Observation) int64
	Process(context.Context, shared.Observation) shared.PluginResult
	PageScripts(shared.RequestSnapshot) []shared.PageScriptInjection
	HandlePageBridge(*http.Request) (*http.Response, bool)
}

type ResponseCaptureStore interface {
	Begin(string, *http.Response) (io.WriteCloser, error)
}

type requestPluginState struct {
	pageScripts []shared.PageScriptInjection
}

type Engine struct {
	Proxy        *goproxy.ProxyHttpServer
	certificates CertificateProvider
	settings     SettingsProvider
	rules        RuleMatcher
	plugins      PluginProcessor
	captures     ResponseCaptureStore
}

func New(certificates CertificateProvider, settings SettingsProvider, rules RuleMatcher, plugins PluginProcessor) *Engine {
	return &Engine{certificates: certificates, settings: settings, rules: rules, plugins: plugins}
}

func (p *Engine) SetCaptureStore(captures ResponseCaptureStore) { p.captures = captures }

func (p *Engine) Start() error {
	if err := p.setCA(); err != nil {
		return err
	}

	p.Proxy = goproxy.NewProxyHttpServer()
	//p.Proxy.KeepDestinationHeaders = true
	//p.Proxy.Verbose = false
	p.SetTransport()
	//p.Proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	p.Proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if p.rules.ShouldMITM(host) {
			return goproxy.MitmConnect, host
		}
		return goproxy.OkConnect, host
	})

	p.Proxy.OnRequest().DoFunc(p.httpRequestEvent)
	p.Proxy.OnResponse().DoFunc(p.httpResponseEvent)
	return nil
}

func (p *Engine) setCA() error {
	certificatePEM, privateKeyPEM := p.certificates()
	ca, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
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

func (p *Engine) SetTransport() {
	if p.Proxy == nil {
		return
	}
	settings := p.settings()
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

	if settings.UpstreamProxy != "" && settings.OpenProxy && !strings.Contains(settings.UpstreamProxy, settings.Port) {
		proxyURL, err := url.Parse(settings.UpstreamProxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			p.Proxy.ConnectDial = p.Proxy.NewConnectDialToProxy(settings.UpstreamProxy)
		}
	}
	p.Proxy.Tr = transport
}

func (p *Engine) httpRequestEvent(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if response, handled := p.plugins.HandlePageBridge(r); handled {
		return r, response
	}
	obs := requestObservation(r)
	pageScripts := p.plugins.PageScripts(obs.Request)
	if len(pageScripts) > 0 {
		ctx.UserData = &requestPluginState{pageScripts: pageScripts}
		r.Header.Set("Accept-Encoding", "identity")
	}
	if limit := p.plugins.BodyLimit(obs); limit > 0 && r.Body != nil {
		body, truncated := readBodyPrefix(&r.Body, limit)
		obs.Request.Body = string(body)
		obs.Request.Truncated = truncated
	}
	result := p.plugins.Process(r.Context(), obs)
	if result.SyntheticResponse != nil {
		return r, buildSyntheticResponse(r, result.SyntheticResponse)
	}
	return r, nil
}

func (p *Engine) httpResponseEvent(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.Request == nil {
		return resp
	}

	obs := responseObservation(resp)
	if limit := p.plugins.BodyLimit(obs); limit > 0 && resp.Body != nil {
		body, truncated := readResponseBodyForPlugins(resp, limit)
		obs.Response.Body = string(body)
		obs.Response.Truncated = truncated
	}
	result := p.plugins.Process(resp.Request.Context(), obs)
	if result.Patch != nil {
		applyHTTPResponsePatch(resp, result.Patch)
	}
	if state, ok := ctx.UserData.(*requestPluginState); ok && len(state.pageScripts) > 0 {
		injectPageScripts(resp, state.pageScripts)
	}
	if p.captures != nil && len(result.Captures) > 0 {
		attachResponseCaptures(resp, result.Captures, p.captures)
	}
	return resp
}

func requestObservation(request *http.Request) shared.Observation {
	return shared.Observation{
		Stage: shared.StageRequest,
		Request: shared.RequestSnapshot{
			Method:  request.Method,
			URL:     request.URL.String(),
			Host:    request.Host,
			Path:    request.URL.Path,
			Headers: cloneHeaders(request.Header),
		},
	}
}

func responseObservation(response *http.Response) shared.Observation {
	obs := requestObservation(response.Request)
	obs.Stage = shared.StageResponse
	obs.Response = &shared.ResponseSnapshot{
		StatusCode:  response.StatusCode,
		Headers:     cloneHeaders(response.Header),
		ContentType: response.Header.Get("Content-Type"),
	}
	return obs
}

func cloneHeaders(headers http.Header) shared.HeaderMap {
	cloned := make(shared.HeaderMap, len(headers))
	for key, values := range headers {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

type prefixedReadCloser struct {
	io.Reader
	closer io.Closer
}

func (r *prefixedReadCloser) Close() error { return r.closer.Close() }

func readBodyPrefix(body *io.ReadCloser, limit int64) ([]byte, bool) {
	if body == nil || *body == nil || limit <= 0 {
		return nil, false
	}
	original := *body
	prefix, err := io.ReadAll(io.LimitReader(original, limit+1))
	truncated := int64(len(prefix)) > limit
	if err != nil {
		truncated = true
	}
	visible := prefix
	if truncated {
		visible = prefix[:min(int64(len(prefix)), limit)]
	}
	*body = &prefixedReadCloser{Reader: io.MultiReader(bytes.NewReader(prefix), original), closer: original}
	return visible, truncated
}

func readResponseBodyForPlugins(response *http.Response, limit int64) ([]byte, bool) {
	body, truncated := readBodyPrefix(&response.Body, limit)
	if truncated || !strings.EqualFold(response.Header.Get("Content-Encoding"), "gzip") {
		return body, truncated
	}
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return body, false
	}
	defer reader.Close()
	decoded, err := io.ReadAll(io.LimitReader(reader, limit+1))
	decodedTruncated := err != nil || int64(len(decoded)) > limit
	if int64(len(decoded)) > limit {
		decoded = decoded[:limit]
	}
	return decoded, decodedTruncated
}

func applyHTTPResponsePatch(response *http.Response, patch *shared.ResponsePatch) {
	if patch.StatusCode != 0 {
		response.StatusCode = patch.StatusCode
		response.Status = fmt.Sprintf("%d %s", patch.StatusCode, http.StatusText(patch.StatusCode))
	}
	for key, value := range patch.Headers {
		response.Header.Set(key, value)
	}
	if patch.Body != nil {
		body := []byte(*patch.Body)
		_ = response.Body.Close()
		response.Body = io.NopCloser(bytes.NewReader(body))
		response.ContentLength = int64(len(body))
		response.Header.Set("Content-Length", strconv.Itoa(len(body)))
		response.Header.Del("Content-Encoding")
		response.Header.Del("ETag")
		response.Header.Del("Content-MD5")
	}
}

func buildSyntheticResponse(request *http.Request, synthetic *shared.SyntheticResponse) *http.Response {
	status := synthetic.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	body := []byte(synthetic.Body)
	response := &http.Response{
		Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), StatusCode: status,
		Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)), Request: request,
	}
	for key, value := range synthetic.Headers {
		response.Header.Set(key, value)
	}
	response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return response
}
