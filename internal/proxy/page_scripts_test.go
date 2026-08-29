package proxy

import (
	"io"
	"net/http"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
)

func TestInjectPageScriptsCopiesNonceAndPreservesBody(t *testing.T) {
	body := `<html><head><script nonce="abc123">window.before=true</script></head><body>ok</body></html>`
	response := &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)),
	}
	scripts := []shared.PageScriptInjection{{PluginID: "example.page", PluginVersion: "1.0.0", ScriptID: "hook", Source: `window.injected = pageApi.scriptId`, Frames: "top"}}
	if !injectPageScripts(response, scripts) {
		t.Fatal("expected page script injection")
	}
	patched, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(patched)
	if !strings.Contains(text, `data-res-downloader-page-script="example.page:hook" nonce="abc123"`) {
		t.Fatalf("injected tag does not copy nonce: %s", text)
	}
	if !strings.Contains(text, body[strings.Index(body, `<script nonce=`):]) {
		t.Fatal("original page body was not preserved")
	}
}

func TestInjectPageScriptsDoesNotWeakenCSP(t *testing.T) {
	body := `<html><head></head><body>ok</body></html>`
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":            []string{"text/html"},
			"Content-Security-Policy": []string{"script-src 'self'"},
		},
		Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)),
	}
	if injectPageScripts(response, []shared.PageScriptInjection{{PluginID: "example.page", ScriptID: "hook", Source: `window.injected = true`}}) {
		t.Fatal("injection should be skipped when CSP blocks inline scripts and no nonce is available")
	}
	preserved, _ := io.ReadAll(response.Body)
	if string(preserved) != body {
		t.Fatal("skipped injection changed the response body")
	}
}

func TestBuildPageScriptTagUsesRealNewlines(t *testing.T) {
	tag := buildPageScriptTag(shared.PageScriptInjection{
		PluginID: "example.page",
		ScriptID: "hook",
		Source:   `window.injected = true`,
	}, "abc123")

	if strings.Contains(tag, `\n}).call(window,pageApi)`) || strings.Contains(tag, `})();\n//# sourceURL=`) {
		t.Fatal("page script wrapper contains literal newline escapes outside JavaScript strings")
	}
	if !strings.Contains(tag, "\n}).call(window,pageApi)") {
		t.Fatal("page script source and wrapper are not separated by a real newline")
	}
	if !strings.Contains(tag, "})();\n//# sourceURL=res-downloader://example.page/hook.js\n</script>") {
		t.Fatal("page script wrapper does not contain real sourceURL newlines")
	}
}

func TestBuildPageScriptTagExposesScopedBinaryCapture(t *testing.T) {
	tag := buildPageScriptTag(shared.PageScriptInjection{
		PluginID: "example.page", ScriptID: "hook", Bridge: true, Capture: true,
		PageSessionID: "session", BridgeToken: "token",
	}, "")

	if !strings.Contains(tag, "captureEnabled=true") || !strings.Contains(tag, `capture:Object.freeze({start:function(key)`) {
		t.Fatal("page script wrapper does not expose binary capture")
	}
	if !strings.Contains(tag, `bridgeBase+"capture-"+action`) {
		t.Fatal("page script capture does not use the scoped bridge URL")
	}
}
