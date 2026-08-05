package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// 复现 bug: 服务器对无 Range 的请求也回 206(部分内容), 单段下载应接受而非报错
func TestDownloadAccepts206WithoutRange(t *testing.T) {
	globalConfig = &Config{}
	globalLogger = NewLogger(false, "")
	body := make([]byte, 1024*100) // 100KB
	for i := range body {
		body[i] = byte(i % 251)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprint(len(body))) // 不声明 Accept-Ranges → 单段模式
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(body)-1, len(body)))
		w.WriteHeader(http.StatusPartialContent) // 206
		w.Write(body)
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "out.mp4")
	fd := NewFileDownloader(srv.URL, out, 1, map[string]string{})
	if err := fd.Start(); err != nil {
		t.Fatalf("206 下载失败(回归): %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(body) {
		t.Fatalf("文件长度 %d != 预期 %d", len(got), len(body))
	}
	for i := range got {
		if got[i] != body[i] {
			t.Fatalf("第 %d 字节不一致", i)
		}
	}
}
