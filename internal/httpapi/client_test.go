package httpapi

import (
	"net/http"
	"net/url"
	"res-downloader/internal/config"
	"testing"
)

func TestPreviewHTTPClientIgnoresEnvironmentProxyByDefault(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:19999")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:19999")
	server := &Server{config: &config.Config{}}
	transport, ok := server.previewHTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatal("preview client does not use an HTTP transport")
	}
	request := &http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}
	proxyURL, err := transport.Proxy(request)
	if err != nil {
		t.Fatalf("resolve preview proxy: %v", err)
	}
	if proxyURL != nil {
		t.Fatalf("preview client unexpectedly inherited environment proxy %q", proxyURL)
	}
}

func TestPreviewHTTPClientUsesExplicitDownloadProxy(t *testing.T) {
	server := &Server{config: &config.Config{
		DownloadProxy: true,
		UpstreamProxy: "http://127.0.0.1:17890",
	}}
	transport := server.previewHTTPClient().Transport.(*http.Transport)
	request := &http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}
	proxyURL, err := transport.Proxy(request)
	if err != nil {
		t.Fatalf("resolve explicit preview proxy: %v", err)
	}
	if proxyURL.String() != "http://127.0.0.1:17890" {
		t.Fatalf("unexpected proxy URL %q", proxyURL)
	}
}

func TestApplyPreviewRequestHeadersDropsTransportAndCacheValidators(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://example.com/live.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	applyPreviewRequestHeaders(request, map[string]string{
		"If-None-Match":     `"cached"`,
		"If-Modified-Since": "yesterday",
		"Range":             "bytes=0-10",
		"Referer":           "https://example.com/page",
		"Authorization":     "Bearer token",
	})
	for _, key := range []string{"If-None-Match", "If-Modified-Since", "Range"} {
		if request.Header.Get(key) != "" {
			t.Fatalf("forbidden header %s was forwarded", key)
		}
	}
	if request.Header.Get("Referer") == "" || request.Header.Get("Authorization") == "" {
		t.Fatal("required captured headers were not forwarded")
	}
}
