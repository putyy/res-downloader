package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"res-downloader/core/shared"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type respData map[string]interface{}

// play serves a tiny HTML page that uses flv.js to play the /api/preview stream.
func (h *HttpServer) play(w http.ResponseWriter, r *http.Request) {
    // Build the preview URL by preserving the original query string
    qs := r.URL.RawQuery
    // Force 127.0.0.1 here because the API router only handles Host=="127.0.0.1:port"
    previewURL := fmt.Sprintf("http://127.0.0.1:%s/api/preview?%s", globalConfig.Port, qs)
    html := `<!doctype html><html><head><meta charset="utf-8"><title>FLV Preview</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>html,body{margin:0;height:100%;background:#000;color:#0f0;font-family:system-ui}#bar{position:fixed;left:0;top:0;right:0;background:#111;padding:6px 8px;font-size:12px;z-index:9}#vwrap{position:absolute;left:0;top:28px;right:0;bottom:0}#v{width:100%;height:100%;background:#000}</style>
<script src="https://cdn.jsdelivr.net/npm/mpegts.js@1.7.3/dist/mpegts.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/flv.js@1.6.2/dist/flv.min.js"></script>
</head><body>
<div id="bar">
  <button id="btn">开始播放</button>
  <button id="probe">测试拉流</button>
  <span id="st">初始化...</span>
  <span style="opacity:.6"> | 如果不动，请按“开始播放”或查看控制台(Alt+Cmd+I)</span>
  <div>preview: <code id="u"></code></div>
</div>
<div id="vwrap"><video id="v" controls muted></video></div>
<script>
(function(){
  var url = PLACEHOLDER_URL; document.getElementById('u').textContent=url;
  var btn = document.getElementById('btn'); var probe = document.getElementById('probe'); var st = document.getElementById('st');
  function log(){ console.log.apply(console, arguments); st.textContent = Array.from(arguments).join(' '); }
  function start(){
    var useMpegts = !!(window.mpegts && mpegts.isSupported && mpegts.isSupported());
    var useFlvjs = !!(window.flvjs && flvjs.isSupported && flvjs.isSupported());
    if(!useMpegts && !useFlvjs){ st.textContent='MSE 不受支持或播放器加载失败(建议 Chrome/Edge 最新版)'; return }
    console.log('MediaSource:', typeof MediaSource, 'WebKitMediaSource:', typeof window.WebKitMediaSource);
    try { if(useFlvjs) console.log('flv features:', flvjs.getFeatureList && flvjs.getFeatureList()); } catch(e) {}
    var v=document.getElementById('v');
    var p;
    try {
      if(useMpegts){
        p = mpegts.createPlayer({type:'flv', isLive:true, url:url}, {enableWorker:true, stashInitialSize:128});
        p.on(mpegts.Events.ERROR, function(t, d){ console.error('mpegts ERROR', t, d); st.textContent='错误:'+t; });
        p.on(mpegts.Events.STATS_INFO, function(s){ /* no-op */ });
      } else {
        p = flvjs.createPlayer({type:'flv', isLive:true, url:url}, {enableWorker:true, stashInitialSize:128});
        p.on(flvjs.Events.ERROR, function(t, d){ console.error('flv.js ERROR', t, d); st.textContent='错误:'+t; });
        p.on(flvjs.Events.LOADING_COMPLETE, function(){ log('loading complete'); });
        p.on(flvjs.Events.MEDIA_INFO, function(mi){ log('media info', mi.mimeType); });
        p.on(flvjs.Events.METADATA_ARRIVED, function(){ log('metadata'); });
      }
      p.attachMediaElement(v); p.load();
    } catch(e){ console.error('player init failed', e); st.textContent = '初始化失败: '+ e; return }
    var pr = v.play(); if(pr && pr.catch){ pr.catch(function(e){ console.warn('autoplay blocked', e); st.textContent='已加载，点击播放按钮或视频开始'; }); }
  }
  // 预探测 HEAD
  fetch(url, {method:'HEAD'}).then(function(r){ log('HEAD', r.status); }).catch(function(e){ console.warn('HEAD failed', e); });
  btn.addEventListener('click', start);
  probe.addEventListener('click', function(){
    var total = 0; log('开始测试拉流...');
    fetch(url).then(function(r){
      if(!r.body){ log('不支持 ReadableStream'); return }
      var reader = r.body.getReader();
      function pump(){
        reader.read().then(function(res){
          if(res.done){ log('测试结束，总字节:', total); return }
          total += res.value.byteLength; st.textContent = '已接收字节: '+ total; pump();
        }).catch(function(e){ console.error('测试拉流错误', e); });
      }
      pump();
    }).catch(function(e){ console.error('fetch 失败', e); });
  });
  // 尝试自动开始
  setTimeout(start, 10);
})();
</script>
</body></html>`
    // Safely inject URL without fmt formatting conflicts
    quoted := strconv.Quote(previewURL)
    html = strings.ReplaceAll(html, "PLACEHOLDER_URL", quoted)
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    _, _ = w.Write([]byte(html))
}

type ResponseData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type HttpServer struct{}

func initHttpServer() *HttpServer {
	if httpServerOnce == nil {
		httpServerOnce = &HttpServer{}
	}
	return httpServerOnce
}

func (h *HttpServer) run() {
	listener, err := net.Listen("tcp", globalConfig.Host+":"+globalConfig.Port)
	if err != nil {
		globalLogger.Err(err)
		log.Fatalf("Service cannot start: %v", err)
	}
	fmt.Println("Service started, listening http://" + globalConfig.Host + ":" + globalConfig.Port)
	if err1 := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "127.0.0.1:"+globalConfig.Port && HandleApi(w, r) {

		} else {
			proxyOnce.Proxy.ServeHTTP(w, r) // 代理
		}
	})); err1 != nil {
		globalLogger.Err(err1)
		fmt.Printf("Service startup exception: %v", err1)
	}
}

func (h *HttpServer) downCert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-x509-ca-data")
	w.Header().Set("Content-Disposition", "attachment;filename=res-downloader-public.crt")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(appOnce.PublicCrt)))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, io.NopCloser(bytes.NewReader(appOnce.PublicCrt)))
}

func (h *HttpServer) preview(w http.ResponseWriter, r *http.Request) {
	log.Printf("[preview] hit %s %s from %s", r.Method, r.URL.String(), r.RemoteAddr)
	realURL := r.URL.Query().Get("url")
	if realURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		log.Printf("[preview] missing url param")
		return
	}
	realURL, _ = url.QueryUnescape(realURL)
	parsedURL, err := url.Parse(realURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		log.Printf("[preview] NewRequest error: %v", err)
		return
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}

    // set default UA
    if ua := globalConfig.UserAgent; ua != "" {
        request.Header.Set("User-Agent", ua)
    }
    // Tentative default Referer/Origin to target origin; may be overridden by provided headers
    if parsedURL.Scheme != "" && parsedURL.Host != "" {
        request.Header.Set("Referer", parsedURL.Scheme+"://"+parsedURL.Host+"/")
        request.Header.Set("Origin", parsedURL.Scheme+"://"+parsedURL.Host)
    }

    // optional headers from query param (JSON: map[string][]string)
    if raw := r.URL.Query().Get("headers"); raw != "" {
        if decoded, err := url.QueryUnescape(raw); err == nil {
            var m map[string][]string
            if err := json.Unmarshal([]byte(decoded), &m); err == nil {
                useKeys := strings.TrimSpace(globalConfig.UseHeaders)
                hopByHop := map[string]struct{}{
                    "Connection": {}, "Proxy-Connection": {}, "Keep-Alive": {},
                    "Proxy-Authenticate": {}, "Proxy-Authorization": {},
                    "TE": {}, "Trailer": {}, "Transfer-Encoding": {}, "Upgrade": {},
                    // not set Host manually; http.Client sets it correctly
                    "Host": {},
                }
                forwarded := make([]string, 0, len(m))
                for k, arr := range m {
                    if len(arr) == 0 { continue }
                    if _, skip := hopByHop[http.CanonicalHeaderKey(k)]; skip { continue }
                    if useKeys == "*" || strings.Contains(useKeys, k) {
                        request.Header.Set(k, arr[0])
                        forwarded = append(forwarded, k)
                    }
                }
                if len(forwarded) > 0 {
                    log.Printf("[preview] forwarded headers: %s", strings.Join(forwarded, ","))
                }
            }
        }
    }

    // Sensible defaults if still missing
    if request.Header.Get("Accept") == "" {
        request.Header.Set("Accept", "*/*")
    }
    if request.Header.Get("Accept-Language") == "" {
        request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
    }
    // For streaming avoid gzip to keep byte-range friendly
    if request.Header.Get("Accept-Encoding") == "" {
        request.Header.Set("Accept-Encoding", "identity")
    }

    // build client with sane defaults and optional upstream proxy
    transport := &http.Transport{
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * 1e9, // 90s
    }
    if globalConfig != nil && globalConfig.DownloadProxy && globalConfig.UpstreamProxy != "" && !strings.Contains(globalConfig.UpstreamProxy, globalConfig.Port) {
        if proxyURL, err := url.Parse(globalConfig.UpstreamProxy); err == nil {
            transport.Proxy = http.ProxyURL(proxyURL)
        }
    }
    client := &http.Client{Transport: transport, Timeout: 60 * 1e9} // 60s

	resp, err := client.Do(request)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		log.Printf("[preview] upstream do error: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[preview] upstream status: %d content-type: %s", resp.StatusCode, resp.Header.Get("Content-Type"))

    // Decide Content-Type for inline playback
    ct := resp.Header.Get("Content-Type")
    if ct == "" || strings.Contains(strings.ToLower(ct), "octet-stream") {
        if strings.HasSuffix(strings.ToLower(parsedURL.Path), ".flv") {
            ct = "video/x-flv"
        }
    }
    if ct != "" {
        w.Header().Set("Content-Type", ct)
    }
    // Avoid caching and relax CORS for desktop webview
    w.Header().Set("Cache-Control", "no-store")
    w.Header().Set("Pragma", "no-cache")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Headers", "*")
    w.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,OPTIONS")
    w.Header().Set("Content-Disposition", "inline")
    if v := resp.Header.Get("Content-Range"); v != "" {
        w.Header().Set("Content-Range", v)
    }
    if v := resp.Header.Get("Content-Length"); v != "" {
        w.Header().Set("Content-Length", v)
    }

    // For HEAD request, just send headers + status
    if r.Method == http.MethodHead {
        w.WriteHeader(resp.StatusCode)
        log.Printf("[preview] HEAD responded status: %d", resp.StatusCode)
        return
    }

    // Write status once, then stream body. If copy fails, just log to avoid double WriteHeader.
    w.WriteHeader(resp.StatusCode)
    if _, err = io.Copy(w, resp.Body); err != nil {
        // client may cancel/close connection during live streaming; avoid writing headers again
        log.Printf("[preview] stream copy error: %v", err)
    }
}

func (h *HttpServer) send(t string, data interface{}) {
	jsonData, err := json.Marshal(map[string]interface{}{
		"type": t,
		"data": data,
	})
	if err != nil {
		fmt.Println("Error converting map to JSON:", err)
		return
	}
	runtime.EventsEmit(appOnce.ctx, "event", string(jsonData))
}

func (h *HttpServer) writeJson(w http.ResponseWriter, data *ResponseData) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		globalLogger.Err(err)
	}
}

func (h *HttpServer) error(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}

	if len(args) > 0 {
		message = args[0].(string)
	}
	if len(args) > 1 {
		data = args[1]
	}
	h.writeJson(w, h.buildResp(0, message, data))
}

func (h *HttpServer) success(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}

	if len(args) > 0 {
		data = args[0]
	}

	if len(args) > 1 {
		message = args[1].(string)
	}
	h.writeJson(w, h.buildResp(1, message, data))
}

func (h *HttpServer) buildResp(code int, message string, data interface{}) *ResponseData {
	return &ResponseData{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func (h *HttpServer) openDirectoryDialog(w http.ResponseWriter, r *http.Request) {
	folder, err := runtime.OpenDirectoryDialog(appOnce.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: "",
		Title:            "Select a folder",
	})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"folder": folder,
	})
}

func (h *HttpServer) openFileDialog(w http.ResponseWriter, r *http.Request) {
	filePath, err := runtime.OpenFileDialog(appOnce.ctx, runtime.OpenDialogOptions{
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Videos (*.mov;*.mp4)",
				Pattern:     "*.mp4",
			},
		},
		Title: "Select a file",
	})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"file": filePath,
	})
}

func (h *HttpServer) openFolder(w http.ResponseWriter, r *http.Request) {
	var data struct {
		FilePath string `json:"filePath"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil && data.FilePath == "" {
		return
	}

	err = shared.OpenFolder(data.FilePath)
	if err != nil {
		globalLogger.Err(err)
		h.error(w, err.Error())
		return
	}
	h.success(w)
}

func (h *HttpServer) install(w http.ResponseWriter, r *http.Request) {
	if appOnce.isInstall() {
		h.success(w, respData{
			"isPass": systemOnce.Password == "",
		})
		return
	}

	out, err := appOnce.installCert()
	if err != nil {
		h.error(w, err.Error()+"\n"+out, respData{
			"isPass": systemOnce.Password == "",
		})
		return
	}

	h.success(w, respData{
		"isPass": systemOnce.Password == "",
	})
}

func (h *HttpServer) setSystemPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Password string `json:"password"`
		IsCache  bool   `json:"isCache"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	systemOnce.SetPassword(data.Password, data.IsCache)
	h.success(w)
}

func (h *HttpServer) openSystemProxy(w http.ResponseWriter, r *http.Request) {
	err := appOnce.OpenSystemProxy()
	if err != nil {
		h.error(w, err.Error(), respData{
			"value": appOnce.IsProxy,
		})
		return
	}
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) unsetSystemProxy(w http.ResponseWriter, r *http.Request) {
	err := appOnce.UnsetSystemProxy()
	if err != nil {
		h.error(w, err.Error(), respData{
			"value": appOnce.IsProxy,
		})
		return
	}
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) isProxy(w http.ResponseWriter, r *http.Request) {
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) appInfo(w http.ResponseWriter, r *http.Request) {
	h.success(w, appOnce)
}

func (h *HttpServer) getConfig(w http.ResponseWriter, r *http.Request) {
	h.success(w, globalConfig)
}

func (h *HttpServer) setConfig(w http.ResponseWriter, r *http.Request) {
	var data Config
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	globalConfig.setConfig(data)
	h.success(w)
}

func (h *HttpServer) setType(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Type string `json:"type"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil {
		if data.Type != "" {
			resourceOnce.setResType(strings.Split(data.Type, ","))
		} else {
			resourceOnce.setResType([]string{})
		}
	}

	h.success(w)
}

func (h *HttpServer) clear(w http.ResponseWriter, r *http.Request) {
	resourceOnce.clear()
	h.success(w)
}

func (h *HttpServer) delete(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Sign string `json:"sign"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil && data.Sign != "" {
		resourceOnce.delete(data.Sign)
	}
	h.success(w)
}

func (h *HttpServer) download(w http.ResponseWriter, r *http.Request) {
	var data struct {
		shared.MediaInfo
		DecodeStr string `json:"decodeStr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	resourceOnce.download(data.MediaInfo, data.DecodeStr)
	h.success(w)
}

func (h *HttpServer) wxFileDecode(w http.ResponseWriter, r *http.Request) {
	var data struct {
		shared.MediaInfo
		Filename  string `json:"filename"`
		DecodeStr string `json:"decodeStr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	savePath, err := resourceOnce.wxFileDecode(data.MediaInfo, data.Filename, data.DecodeStr)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"save_path": savePath,
	})
}

func (h *HttpServer) batchExport(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	fileName := filepath.Join(globalConfig.SaveDirectory, "res-downloader-"+shared.GetCurrentDateTimeFormatted()+".txt")
	err := os.WriteFile(fileName, []byte(data.Content), 0644)
	if err != nil {
		h.error(w, err.Error())
		return
	}

	_ = shared.OpenFolder(fileName)
	h.success(w, respData{
		"file_name": fileName,
	})
}
