package core

import (
	"encoding/json"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	sysRuntime "runtime"
	"strings"
)

type ResponseData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type HttpServer struct {
	broadcast chan []byte
}

func initHttpServer() *HttpServer {
	if httpServerOnce == nil {
		httpServerOnce = &HttpServer{
			broadcast: make(chan []byte),
		}
	}
	return httpServerOnce
}

func (h *HttpServer) run() {
	listener, err := net.Listen("tcp", globalConfig.Host+":"+globalConfig.Port)
	if err != nil {
		log.Fatalf("无法启动监听: %v", err)
	}
	go h.handleMessages()
	fmt.Println("服务已启动，监听 http://" + globalConfig.Host + ":" + globalConfig.Port)
	if err := http.Serve(listener, proxyOnce.Proxy); err != nil {
		fmt.Printf("服务器异常: %v", err)
	}
}

func (h *HttpServer) preview(w http.ResponseWriter, r *http.Request) {
	realURL := r.URL.Query().Get("url")
	if realURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
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
		return
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}

	//request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")
	//request.Header.Set("Referer", parsedURL.Scheme+"://"+parsedURL.Host+"/")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)

	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		w.Header().Set("Content-Range", contentRange)
	}

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Failed to serve the resource", http.StatusInternalServerError)
	}
	return
}

func (h *HttpServer) handleMessages() {
	for {
		msg := <-h.broadcast
		runtime.EventsEmit(appOnce.ctx, "event", string(msg))
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
	h.broadcast <- jsonData
}

func (h *HttpServer) writeJson(w http.ResponseWriter, data ResponseData) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		globalLogger.err(err)
	}
}

func (h *HttpServer) openDirectoryDialog(w http.ResponseWriter, r *http.Request) {
	folder, err := runtime.OpenDirectoryDialog(appOnce.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: "",
		Title:            "Select a folder",
	})
	if err != nil {
		h.writeJson(w, ResponseData{Code: 0, Message: err.Error()})
		return
	}
	h.writeJson(w, ResponseData{
		Code: 1,
		Data: map[string]interface{}{
			"folder": folder,
		},
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
		h.writeJson(w, ResponseData{Code: 0, Message: err.Error()})
		return
	}
	h.writeJson(w, ResponseData{
		Code: 1,
		Data: map[string]interface{}{
			"file": filePath,
		},
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

	filePath := data.FilePath
	var cmd *exec.Cmd

	switch sysRuntime.GOOS {
	case "darwin":
		// macOS
		cmd = exec.Command("open", "-R", filePath)
	case "windows":
		// Windows
		cmd = exec.Command("explorer", "/select,", filePath)
	case "linux":
		// linux
		// 尝试使用不同的文件管理器
		cmd = exec.Command("nautilus", filePath) // 尝试Nautilus
		if err := cmd.Start(); err != nil {
			cmd = exec.Command("thunar", filePath) // 尝试Thunar
			if err := cmd.Start(); err != nil {
				cmd = exec.Command("dolphin", filePath) // 尝试Dolphin
				if err := cmd.Start(); err != nil {
					cmd = exec.Command("pcmanfm", filePath) // 尝试PCManFM
					if err := cmd.Start(); err != nil {
						globalLogger.err(err)
						h.writeJson(w, ResponseData{Code: 0, Message: err.Error()})
						return
					}
				}
			}
		}
	default:
		h.writeJson(w, ResponseData{Code: 0, Message: "unsupported platform"})
		return
	}

	err = cmd.Start()
	if err != nil {
		globalLogger.err(err)
		h.writeJson(w, ResponseData{Code: 0, Message: err.Error()})
		return
	}
	h.writeJson(w, ResponseData{Code: 1})
}

func (h *HttpServer) openSystemProxy(w http.ResponseWriter, r *http.Request) {
	appOnce.OpenSystemProxy()
	h.writeJson(w, ResponseData{
		Code: 1,
		Data: map[string]bool{
			"isProxy": appOnce.IsProxy,
		},
	})
}

func (h *HttpServer) unsetSystemProxy(w http.ResponseWriter, r *http.Request) {
	appOnce.UnsetSystemProxy()
	h.writeJson(w, ResponseData{
		Code: 1,
		Data: map[string]bool{
			"isProxy": appOnce.IsProxy,
		},
	})
}

func (h *HttpServer) isProxy(w http.ResponseWriter, r *http.Request) {
	h.writeJson(w, ResponseData{
		Code: 1,
		Data: map[string]interface{}{
			"isProxy": appOnce.IsProxy,
		},
	})
}

func (h *HttpServer) appInfo(w http.ResponseWriter, r *http.Request) {
	h.writeJson(w, ResponseData{
		Code: 1,
		Data: appOnce,
	})
}

func (h *HttpServer) getConfig(w http.ResponseWriter, r *http.Request) {
	h.writeJson(w, ResponseData{
		Code: 1,
		Data: globalConfig,
	})
}

func (h *HttpServer) setConfig(w http.ResponseWriter, r *http.Request) {
	var data Config
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.writeJson(w, ResponseData{Code: 0, Message: err.Error()})
		return
	}
	globalConfig.setConfig(data)
	h.writeJson(w, ResponseData{Code: 1})
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

	h.writeJson(w, ResponseData{Code: 1})
}

func (h *HttpServer) clear(w http.ResponseWriter, r *http.Request) {
	resourceOnce.clear()
	h.writeJson(w, ResponseData{Code: 1})
}

func (h *HttpServer) delete(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Sign string `json:"sign"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil && data.Sign != "" {
		resourceOnce.delete(data.Sign)
	}
	h.writeJson(w, ResponseData{Code: 1})
}

func (h *HttpServer) download(w http.ResponseWriter, r *http.Request) {
	var data struct {
		MediaInfo
		DecodeStr string `json:"decodeStr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.writeJson(w, ResponseData{Code: 0, Message: err.Error()})
		return
	}
	resourceOnce.download(data.MediaInfo, data.DecodeStr)
	h.writeJson(w, ResponseData{Code: 1})
}

func (h *HttpServer) wxFileDecode(w http.ResponseWriter, r *http.Request) {
	var data struct {
		MediaInfo
		Filename  string `json:"filename"`
		DecodeStr string `json:"decodeStr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.writeJson(w, ResponseData{Code: 0, Message: err.Error()})
		return
	}
	savePath, err := resourceOnce.wxFileDecode(data.MediaInfo, data.Filename, data.DecodeStr)
	if err != nil {
		h.writeJson(w, ResponseData{Code: 0, Message: err.Error()})
		return
	}
	h.writeJson(w, ResponseData{
		Code: 1,
		Data: map[string]string{
			"save_path": savePath,
		},
	})
}
