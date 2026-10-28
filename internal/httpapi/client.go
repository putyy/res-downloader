package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

var forbiddenPreviewRequestHeaders = map[string]struct{}{
	"accept-encoding":     {},
	"content-length":      {},
	"host":                {},
	"connection":          {},
	"keep-alive":          {},
	"proxy-connection":    {},
	"transfer-encoding":   {},
	"range":               {},
	"if-match":            {},
	"if-none-match":       {},
	"if-modified-since":   {},
	"if-unmodified-since": {},
	"if-range":            {},
}

// previewHTTPClient deliberately ignores HTTP_PROXY and HTTPS_PROXY inherited
// from the process. Captured CDN URLs are often signed for the current network
// path. A proxy is used only when the user explicitly enables DownloadProxy.
func (h *Server) previewHTTPClient() *http.Client {
	h.previewClientMu.Lock()
	defer h.previewClientMu.Unlock()
	if h.previewClient != nil {
		return h.previewClient
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(request *http.Request) (*url.URL, error) {
		if h == nil || h.config == nil {
			return nil, nil
		}
		snapshot := h.config.Snapshot()
		if !snapshot.DownloadProxy {
			return nil, nil
		}
		configured := strings.TrimSpace(snapshot.UpstreamProxy)
		proxyURL, err := url.Parse(configured)
		if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
			return nil, nil
		}
		return proxyURL, nil
	}
	h.previewClient = &http.Client{Transport: transport}
	return h.previewClient
}

func applyPreviewRequestHeaders(request *http.Request, headers map[string]string) {
	for key, value := range headers {
		if _, forbidden := forbiddenPreviewRequestHeaders[strings.ToLower(strings.TrimSpace(key))]; forbidden {
			continue
		}
		request.Header.Set(key, value)
	}
}
