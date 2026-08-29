package proxy

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"regexp"
	shared "res-downloader/internal/model"
	"strconv"
	"strings"
)

const pageInjectionPrefixLimit = 256 * 1024

var (
	pageScriptNoncePattern = regexp.MustCompile(`(?i)<script\b[^>]*\bnonce\s*=\s*["']([^"']+)["']`)
	pagePolicyNoncePattern = regexp.MustCompile(`(?i)(?:^|[\s;])'nonce-([^'\s;]+)'`)
	pageScriptClosePattern = regexp.MustCompile(`(?i)</script`)
)

func injectPageScripts(response *http.Response, scripts []shared.PageScriptInjection) bool {
	if response == nil || response.Body == nil || response.StatusCode != http.StatusOK || len(scripts) == 0 {
		return false
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if !strings.HasPrefix(contentType, "text/html") || strings.TrimSpace(response.Header.Get("Content-Encoding")) != "" {
		return false
	}

	original := response.Body
	prefix, err := io.ReadAll(io.LimitReader(original, pageInjectionPrefixLimit))
	if err != nil {
		response.Body = &prefixedReadCloser{Reader: io.MultiReader(bytes.NewReader(prefix), original), closer: original}
		return false
	}
	insertAt := pageScriptInsertionOffset(prefix)
	if insertAt < 0 {
		response.Body = &prefixedReadCloser{Reader: io.MultiReader(bytes.NewReader(prefix), original), closer: original}
		return false
	}

	nonce := pageScriptNonce(prefix)
	if nonce == "" {
		nonce = pagePolicyNonce(response.Header.Values("Content-Security-Policy"))
	}
	if nonce == "" && !pageInlineScriptAllowed(response.Header.Values("Content-Security-Policy")) {
		response.Body = &prefixedReadCloser{Reader: io.MultiReader(bytes.NewReader(prefix), original), closer: original}
		return false
	}

	var injected strings.Builder
	for _, script := range scripts {
		injected.WriteString(buildPageScriptTag(script, nonce))
	}
	if injected.Len() == 0 {
		response.Body = &prefixedReadCloser{Reader: io.MultiReader(bytes.NewReader(prefix), original), closer: original}
		return false
	}

	addition := []byte(injected.String())
	patchedPrefix := make([]byte, 0, len(prefix)+len(addition))
	patchedPrefix = append(patchedPrefix, prefix[:insertAt]...)
	patchedPrefix = append(patchedPrefix, addition...)
	patchedPrefix = append(patchedPrefix, prefix[insertAt:]...)
	response.Body = &prefixedReadCloser{Reader: io.MultiReader(bytes.NewReader(patchedPrefix), original), closer: original}
	if response.ContentLength >= 0 {
		response.ContentLength += int64(len(addition))
		response.Header.Set("Content-Length", strconv.FormatInt(response.ContentLength, 10))
	} else {
		response.Header.Del("Content-Length")
	}
	response.Header.Del("ETag")
	response.Header.Del("Content-MD5")
	return true
}

func pageScriptInsertionOffset(prefix []byte) int {
	lower := bytes.ToLower(prefix)
	head := bytes.Index(lower, []byte("<head"))
	if head < 0 {
		return -1
	}
	closing := bytes.IndexByte(prefix[head:], '>')
	if closing < 0 {
		return -1
	}
	return head + closing + 1
}

func pageScriptNonce(prefix []byte) string {
	match := pageScriptNoncePattern.FindSubmatch(prefix)
	if len(match) != 2 {
		return ""
	}
	return string(match[1])
}

func pagePolicyNonce(policies []string) string {
	for _, policy := range policies {
		match := pagePolicyNoncePattern.FindStringSubmatch(policy)
		if len(match) == 2 {
			return match[1]
		}
	}
	return ""
}

func pageInlineScriptAllowed(policies []string) bool {
	if len(policies) == 0 {
		return true
	}
	for _, policy := range policies {
		lower := strings.ToLower(policy)
		if strings.Contains(lower, "'unsafe-inline'") {
			return true
		}
	}
	return false
}

func buildPageScriptTag(script shared.PageScriptInjection, nonce string) string {
	pluginID := jsonString(script.PluginID)
	pluginVersion := jsonString(script.PluginVersion)
	scriptID := jsonString(script.ScriptID)
	sessionID := jsonString(script.PageSessionID)
	bridgeBase := ""
	if script.Bridge && script.PageSessionID != "" && script.BridgeToken != "" {
		bridgeBase = pageBridgeBrowserPrefix + script.PageSessionID + "/" + script.BridgeToken + "/"
	}
	bridgeURL := jsonString(bridgeBase)
	captureEnabled := "false"
	if script.Capture && bridgeBase != "" {
		captureEnabled = "true"
	}
	frames := script.Frames
	if frames == "" {
		frames = "top"
	}
	source := pageScriptClosePattern.ReplaceAllString(script.Source, `<\/script`)

	var tag strings.Builder
	tag.WriteString(`<script data-res-downloader-page-script="`)
	tag.WriteString(html.EscapeString(script.PluginID + ":" + script.ScriptID))
	tag.WriteString(`"`)
	if nonce != "" {
		tag.WriteString(` nonce="`)
		tag.WriteString(html.EscapeString(nonce))
		tag.WriteString(`"`)
	}
	tag.WriteString(`>(function(){"use strict";`)
	tag.WriteString(`var pluginId=` + pluginID + `,pluginVersion=` + pluginVersion + `,scriptId=` + scriptID + `,pageSessionId=` + sessionID + `,bridgeBase=` + bridgeURL + `,captureEnabled=` + captureEnabled + `;`)
	if frames == "top" {
		tag.WriteString(`if(window.top!==window)return;`)
	}
	tag.WriteString(`var listeners=[],nativeFetch=window.fetch&&window.fetch.bind(window),NativeEventSource=window.EventSource;`)
	tag.WriteString(`function captureRequest(action,key,body){if(!captureEnabled||!bridgeBase||!nativeFetch)return Promise.reject(new Error("page capture is unavailable"));if(typeof key!=="string"||!key||key.length>512)return Promise.reject(new Error("capture key is invalid"));var headers={"X-Res-Downloader-Capture-Key":key},options={method:"POST",headers:headers,credentials:"same-origin",cache:"no-store"};if(body!==undefined){if(!(body instanceof ArrayBuffer)&&!ArrayBuffer.isView(body))return Promise.reject(new TypeError("capture body must be binary"));headers["Content-Type"]="application/octet-stream";options.body=body;}return nativeFetch(bridgeBase+"capture-"+action,options).then(function(response){return response.json().then(function(result){if(!response.ok||!result.ok)throw new Error(result.error||"page capture failed");return result;});});}`)
	tag.WriteString(`var pageApi=Object.freeze({pluginId:pluginId,pluginVersion:pluginVersion,scriptId:scriptId,pageSessionId:pageSessionId,`)
	tag.WriteString(`send:function(message){if(!bridgeBase||!nativeFetch)return Promise.reject(new Error("page bridge is unavailable"));var body=JSON.stringify(message);if(body.length>65536)return Promise.reject(new Error("page message is too large"));return nativeFetch(bridgeBase+"message",{method:"POST",headers:{"Content-Type":"application/json"},body:body,credentials:"same-origin",cache:"no-store"}).then(function(response){return response.json();});},`)
	tag.WriteString(`capture:Object.freeze({start:function(key){return captureRequest("start",key);},write:function(key,body){return captureRequest("write",key,body);},complete:function(key){return captureRequest("complete",key);},abort:function(key){return captureRequest("abort",key);}}),`)
	tag.WriteString(`onMessage:function(listener){if(typeof listener!=="function")throw new TypeError("listener must be a function");listeners.push(listener);return function(){var index=listeners.indexOf(listener);if(index>=0)listeners.splice(index,1);};}});`)
	tag.WriteString(`if(bridgeBase&&NativeEventSource){var events=new NativeEventSource(bridgeBase+"events");events.addEventListener("message",function(event){try{var message=JSON.parse(event.data);listeners.slice().forEach(function(listener){try{listener(message);}catch(error){console.warn("res-downloader page listener failed",error);}});}catch(error){console.warn("res-downloader page message is invalid",error);}});window.addEventListener("pagehide",function(){try{if(navigator.sendBeacon)navigator.sendBeacon(bridgeBase+"close","");events.close();}catch(error){}},{once:true});}`)
	tag.WriteString(`Promise.resolve((async function(pageApi){`)
	tag.WriteString(source)
	tag.WriteString("\n}).call(window,pageApi)).catch(function(error){console.warn(\"res-downloader page script failed\",error);});")
	tag.WriteString("})();\n//# sourceURL=res-downloader://" + script.PluginID + "/" + script.ScriptID + ".js\n</script>")
	return tag.String()
}

const pageBridgeBrowserPrefix = "/.well-known/res-downloader/page-bridge/"

func jsonString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
