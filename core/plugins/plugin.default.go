package plugins

import (
	"encoding/json"
	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"net/http"
	"path/filepath"
	"res-downloader/core/shared"
	"strconv"
	"strings"
)

type DefaultPlugin struct {
	bridge *shared.Bridge
}

func (p *DefaultPlugin) SetBridge(bridge *shared.Bridge) {
	p.bridge = bridge
}

func (p *DefaultPlugin) Domains() []string {
	return []string{"default"}
}

func (p *DefaultPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return r, nil
}

// responseSize returns the size of the complete resource. For a 206 response,
// Content-Length is only the size of the requested range, while Content-Range
// carries the full object size (for example: "bytes 0-65535/1234567").
func responseSize(resp *http.Response) float64 {
	if resp.StatusCode == http.StatusPartialContent {
		contentRange := resp.Header.Get("Content-Range")
		if slash := strings.LastIndex(contentRange, "/"); slash >= 0 {
			total := strings.TrimSpace(contentRange[slash+1:])
			if total != "" && total != "*" {
				if value, err := strconv.ParseFloat(total, 64); err == nil {
					return value
				}
			}
		}
	}

	value, _ := strconv.ParseFloat(resp.Header.Get("Content-Length"), 64)
	return value
}

func (p *DefaultPlugin) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.Request == nil || (resp.StatusCode != 200 && resp.StatusCode != 206 && resp.StatusCode != 304) {
		return resp
	}

	classify, suffix := p.bridge.TypeSuffix(resp.Header.Get("Content-Type"))
	if classify == "" {
		return resp
	}

	rawUrl := resp.Request.URL.String()
	isAll, _ := p.bridge.GetResType("all")
	isClassify, _ := p.bridge.GetResType(classify)

	if suffix == "default" {
		ext := filepath.Ext(filepath.Base(strings.Split(strings.Split(rawUrl, "?")[0], "#")[0]))
		if ext != "" {
			suffix = ext
		}
	}

	urlSign := shared.Md5(rawUrl)
	if ok := p.bridge.MediaIsMarked(urlSign); !ok && (isAll || isClassify) {
		value := responseSize(resp)
		id, err := gonanoid.New()
		if err != nil {
			id = urlSign
		}
		res := shared.MediaInfo{
			Id:          id,
			Url:         rawUrl,
			UrlSign:     urlSign,
			CoverUrl:    "",
			Size:        value,
			Domain:      shared.GetTopLevelDomain(rawUrl),
			Classify:    classify,
			Suffix:      suffix,
			Status:      shared.DownloadStatusReady,
			SavePath:    "",
			DecodeKey:   "",
			OtherData:   map[string]string{},
			Description: "",
			ContentType: resp.Header.Get("Content-Type"),
		}

		// Store entire request headers as JSON
		if headers, err := json.Marshal(resp.Request.Header); err == nil {
			res.OtherData["headers"] = string(headers)
		}

		p.bridge.MarkMedia(urlSign)
		go func(res shared.MediaInfo) {
			p.bridge.Send("newResources", res)
		}(res)
	}

	return resp
}
